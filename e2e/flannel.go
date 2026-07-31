//go:build e2e

// Copyright 2026 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

const flannelManifestPath = "../Documentation/kube-flannel.yml"

// appliedObject records enough to delete a previously created object.
type appliedObject struct {
	gvr       schema.GroupVersionResource
	namespace string
	name      string
}

// dynamicResource returns the resource client for a GVR, scoped to a namespace
// when one is set (empty namespace means a cluster-scoped resource).
func dynamicResource(dyn dynamic.Interface, gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if namespace != "" {
		return dyn.Resource(gvr).Namespace(namespace)
	}
	return dyn.Resource(gvr)
}

// installFlannel renders the shipped kube-flannel.yml as a template (patching
// the flannel image, net-conf.json and, for the udp backend, the privileged
// flag) and applies it via the dynamic client. It replaces write-flannel-conf +
// install-flannel. The list of created objects is stored for later deletion.
func (kc *kindCluster) installFlannel(ctx context.Context, backend string, enableNFTables bool) error {
	objs, err := renderFlannelManifest(backend, enableNFTables)
	if err != nil {
		return err
	}

	dyn, err := dynamic.NewForConfig(kc.restCfg)
	if err != nil {
		return err
	}
	mapper, err := kc.restMapper()
	if err != nil {
		return err
	}

	kc.applied = nil
	for _, obj := range objs {
		gvk := obj.GroupVersionKind()
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("mapping %s: %w", gvk, err)
		}
		ns := ""
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			ns = obj.GetNamespace()
		}
		ginkgo.By(fmt.Sprintf("applying %s %s/%s", gvk.Kind, ns, obj.GetName()))
		ri := dynamicResource(dyn, mapping.Resource, ns)
		if _, err := ri.Create(ctx, obj, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("creating %s %s: %w", gvk.Kind, obj.GetName(), err)
			}
			// Resource already exists (e.g. from a previous partial run); update it.
			existing, getErr := ri.Get(ctx, obj.GetName(), metav1.GetOptions{})
			if getErr != nil {
				return fmt.Errorf("getting existing %s %s: %w", gvk.Kind, obj.GetName(), getErr)
			}
			obj.SetResourceVersion(existing.GetResourceVersion())
			if _, err := ri.Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("updating %s %s: %w", gvk.Kind, obj.GetName(), err)
			}
		}
		kc.applied = append(kc.applied, appliedObject{
			gvr:       mapping.Resource,
			namespace: ns,
			name:      obj.GetName(),
		})
	}
	return nil
}

// deleteFlannel removes every object created by installFlannel, mirroring
// delete-flannel.
func (kc *kindCluster) deleteFlannel(ctx context.Context) error {
	dyn, err := dynamic.NewForConfig(kc.restCfg)
	if err != nil {
		return err
	}
	applied := append([]appliedObject(nil), kc.applied...)

	// Delete in reverse creation order.
	for i := len(applied) - 1; i >= 0; i-- {
		o := applied[i]
		if err := dynamicResource(dyn, o.gvr, o.namespace).Delete(ctx, o.name, metav1.DeleteOptions{}); err != nil {
			logf("warning: deleting %s/%s: %v\n", o.namespace, o.name, err)
		}
	}
	if err := kc.waitForFlannelDeletion(ctx, dyn, applied, 2*time.Minute); err != nil {
		return err
	}
	kc.applied = nil
	return nil
}

func (kc *kindCluster) waitForFlannelDeletion(ctx context.Context, dyn dynamic.Interface, applied []appliedObject, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		for _, o := range applied {
			if _, err := dynamicResource(dyn, o.gvr, o.namespace).Get(ctx, o.name, metav1.GetOptions{}); err == nil {
				return false, nil
			} else if !apierrors.IsNotFound(err) {
				return false, err
			}
		}
		return true, nil
	})
}

// waitForFlannelRollout waits for the flannel DaemonSet to be fully available,
// mirroring `kubectl rollout status daemonset/kube-flannel-ds`.
func (kc *kindCluster) waitForFlannelRollout(ctx context.Context, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		ds, err := kc.client.AppsV1().DaemonSets(flannelNamespace).Get(ctx, "kube-flannel-ds", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return daemonSetRolledOut(ds), nil
	})
}

func daemonSetRolledOut(ds *appsv1.DaemonSet) bool {
	s := ds.Status
	if ds.Generation != s.ObservedGeneration {
		return false
	}
	if s.DesiredNumberScheduled == 0 {
		return false
	}
	return s.UpdatedNumberScheduled == s.DesiredNumberScheduled &&
		s.NumberAvailable == s.DesiredNumberScheduled
}

func (kc *kindCluster) restMapper() (meta.RESTMapper, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(kc.restCfg)
	if err != nil {
		return nil, err
	}
	groupResources, err := restmapper.GetAPIGroupResources(dc)
	if err != nil {
		return nil, err
	}
	return restmapper.NewDiscoveryRESTMapper(groupResources), nil
}

// renderFlannelManifest loads the shipped manifest and patches it in place.
func renderFlannelManifest(backend string, enableNFTables bool) ([]*unstructured.Unstructured, error) {
	raw, err := os.ReadFile(flannelManifestPath)
	if err != nil {
		return nil, err
	}
	objs, err := decodeYAMLObjects(raw)
	if err != nil {
		return nil, err
	}

	netConf := fmt.Sprintf(
		`{ "Network": "%s", "Backend": { "Type": "%s" }, "EnableNFTables": %t }`,
		flannelNet, backend, enableNFTables,
	)

	for _, obj := range objs {
		switch obj.GetKind() {
		case "ConfigMap":
			if obj.GetName() != "kube-flannel-cfg" {
				continue
			}
			if err := unstructured.SetNestedField(obj.Object, netConf, "data", "net-conf.json"); err != nil {
				return nil, err
			}
		case "DaemonSet":
			if err := patchFlannelDaemonSet(obj, backend); err != nil {
				return nil, err
			}
		}
	}
	return objs, nil
}

func patchFlannelDaemonSet(obj *unstructured.Unstructured, backend string) error {
	// containers[0].image
	containers, _, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	if err != nil {
		return err
	}
	if len(containers) > 0 {
		c, ok := containers[0].(map[string]interface{})
		if !ok {
			return fmt.Errorf("containers[0] has unexpected type %T", containers[0])
		}
		c["image"] = flannelImage
		// udp backend needs privileged mode to access /dev/net/tun.
		if backend == "udp" {
			sc, ok := c["securityContext"].(map[string]interface{})
			if !ok {
				sc = map[string]interface{}{}
			}
			sc["privileged"] = true
			c["securityContext"] = sc
		}
	}
	if err := unstructured.SetNestedSlice(obj.Object, containers, "spec", "template", "spec", "containers"); err != nil {
		return err
	}

	// initContainers[1].image (install-cni uses the flannel image)
	initContainers, _, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "initContainers")
	if err != nil {
		return err
	}
	if len(initContainers) > 1 {
		ic, ok := initContainers[1].(map[string]interface{})
		if !ok {
			return fmt.Errorf("initContainers[1] has unexpected type %T", initContainers[1])
		}
		ic["image"] = flannelImage
	}
	return unstructured.SetNestedSlice(obj.Object, initContainers, "spec", "template", "spec", "initContainers")
}

// decodeYAMLObjects splits a multi-document YAML manifest into unstructured objects.
func decodeYAMLObjects(raw []byte) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	reader := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), 4096)
	for {
		obj := &unstructured.Unstructured{}
		if err := reader.Decode(obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(obj.Object) == 0 {
			continue
		}
		objs = append(objs, obj)
	}
	return objs, nil
}
