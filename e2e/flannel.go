//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"

	appsv1 "k8s.io/api/apps/v1"
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
		ns := obj.GetNamespace()
		var ri dynamic.ResourceInterface
		if mapping.Scope.Name() == "namespace" {
			ri = dyn.Resource(mapping.Resource).Namespace(ns)
		} else {
			ri = dyn.Resource(mapping.Resource)
		}
		By(fmt.Sprintf("applying %s %s/%s", gvk.Kind, ns, obj.GetName()))
		if _, err := ri.Create(ctx, obj, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("creating %s %s: %w", gvk.Kind, obj.GetName(), err)
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
	// Delete in reverse creation order.
	for i := len(kc.applied) - 1; i >= 0; i-- {
		o := kc.applied[i]
		var ri dynamic.ResourceInterface
		if o.namespace != "" {
			ri = dyn.Resource(o.gvr).Namespace(o.namespace)
		} else {
			ri = dyn.Resource(o.gvr)
		}
		if err := ri.Delete(ctx, o.name, metav1.DeleteOptions{}); err != nil {
			fmt.Fprintf(GinkgoWriter, "warning: deleting %s/%s: %v\n", o.namespace, o.name, err)
		}
	}
	kc.applied = nil
	return nil
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
		c := containers[0].(map[string]interface{})
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
		ic := initContainers[1].(map[string]interface{})
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
