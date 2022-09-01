// Copyright 2016 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/subnet"
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	log "k8s.io/klog"
)

var (
	ErrUnimplemented = errors.New("unimplemented")
)

const (
	resyncPeriod              = 5 * time.Minute
	nodeControllerSyncTimeout = 10 * time.Minute
)

type kubeSubnetManager struct {
	enableIPv4                bool
	enableIPv6                bool
	annotations               annotations
	client                    clientset.Interface
	nodeName                  string
	nodeStore                 listers.NodeLister
	nodeController            cache.Controller
	subnetConf                *subnet.Config
	events                    chan subnet.Event
	setNodeNetworkUnavailable bool
}

func NewSubnetManager(ctx context.Context, apiUrl, kubeconfig, prefix, netConfPath string, setNodeNetworkUnavailable bool) (subnet.Manager, error) {
	var cfg *rest.Config
	var err error
	// Try to build kubernetes config from a master url or a kubeconfig filepath. If neither masterUrl
	// or kubeconfigPath are passed in we fall back to inClusterConfig. If inClusterConfig fails,
	// we fallback to the default config.
	cfg, err = clientcmd.BuildConfigFromFlags(apiUrl, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("fail to create kubernetes config: %v", err)
	}

	c, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize client: %v", err)
	}

	// The kube subnet mgr needs to know the k8s node name that it's running on so it can annotate it.
	// If we're running as a pod then the POD_NAME and POD_NAMESPACE will be populated and can be used to find the node
	// name. Otherwise, the environment variable NODE_NAME can be passed in.
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		podName := os.Getenv("POD_NAME")
		podNamespace := os.Getenv("POD_NAMESPACE")
		if podName == "" || podNamespace == "" {
			return nil, fmt.Errorf("env variables POD_NAME and POD_NAMESPACE must be set")
		}

		pod, err := c.CoreV1().Pods(podNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error retrieving pod spec for '%s/%s': %v", podNamespace, podName, err)
		}
		nodeName = pod.Spec.NodeName
		if nodeName == "" {
			return nil, fmt.Errorf("node name not present in pod spec '%s/%s'", podNamespace, podName)
		}
	}

	netConf, err := os.ReadFile(netConfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read net conf: %v", err)
	}

	sc, err := subnet.ParseConfig(string(netConf))
	if err != nil {
		return nil, fmt.Errorf("error parsing subnet config: %s", err)
	}

	sm, err := newKubeSubnetManager(ctx, c, sc, nodeName, prefix)
	if err != nil {
		return nil, fmt.Errorf("error creating network manager: %s", err)
	}
	sm.setNodeNetworkUnavailable = setNodeNetworkUnavailable
	go sm.Run(context.Background())

	log.Infof("Waiting %s for node controller to sync", nodeControllerSyncTimeout)
	err = wait.Poll(time.Second, nodeControllerSyncTimeout, func() (bool, error) {
		return sm.nodeController.HasSynced(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for nodeController to sync state: %v", err)
	}
	log.Infof("Node controller sync successful")

	return sm, nil
}

func newKubeSubnetManager(ctx context.Context, c clientset.Interface, sc *subnet.Config, nodeName, prefix string) (*kubeSubnetManager, error) {
	var err error
	var ksm kubeSubnetManager
	ksm.annotations, err = newAnnotations(prefix)
	if err != nil {
		return nil, err
	}
	ksm.enableIPv4 = sc.EnableIPv4
	ksm.enableIPv6 = sc.EnableIPv6
	ksm.client = c
	ksm.nodeName = nodeName
	ksm.subnetConf = sc
	scale := 5000
	scaleStr := os.Getenv("EVENT_QUEUE_DEPTH")
	if scaleStr != "" {
		n, err := strconv.Atoi(scaleStr)
		if err != nil {
			return nil, fmt.Errorf("env EVENT_QUEUE_DEPTH=%s format error: %v", scaleStr, err)
		}
		if n > 0 {
			scale = n
		}
	}
	ksm.events = make(chan subnet.Event, scale)
	indexer, controller := cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return ksm.client.CoreV1().Nodes().List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return ksm.client.CoreV1().Nodes().Watch(ctx, options)
			},
		},
		&v1.Node{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ksm.handleAddLeaseEvent(subnet.EventAdded, obj)
			},
			UpdateFunc: ksm.handleUpdateLeaseEvent,
			DeleteFunc: func(obj interface{}) {
				_, isNode := obj.(*v1.Node)
				// We can get DeletedFinalStateUnknown instead of *api.Node here and we need to handle that correctly.
				if !isNode {
					deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						log.Infof("Error received unexpected object: %v", obj)
						return
					}
					node, ok := deletedState.Obj.(*v1.Node)
					if !ok {
						log.Infof("Error deletedFinalStateUnknown contained non-Node object: %v", deletedState.Obj)
						return
					}
					obj = node
				}
				ksm.handleAddLeaseEvent(subnet.EventRemoved, obj)
			},
		},
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	ksm.nodeController = controller
	ksm.nodeStore = listers.NewNodeLister(indexer)
	return &ksm, nil
}

func (ksm *kubeSubnetManager) handleAddLeaseEvent(et subnet.EventType, obj interface{}) {
	n := obj.(*v1.Node)
	if s, ok := n.Annotations[ksm.annotations.SubnetKubeManaged]; !ok || s != "true" {
		return
	}

	l, err := ksm.nodeToLease(*n)
	if err != nil {
		log.Infof("Error turning node %q to lease: %v", n.ObjectMeta.Name, err)
		return
	}
	ksm.events <- subnet.Event{Type: et, Lease: l}
}

func (ksm *kubeSubnetManager) handleUpdateLeaseEvent(oldObj, newObj interface{}) {
	o := oldObj.(*v1.Node)
	n := newObj.(*v1.Node)
	if s, ok := n.Annotations[ksm.annotations.SubnetKubeManaged]; !ok || s != "true" {
		return
	}
	var changed = true
	if ksm.enableIPv4 && o.Annotations[ksm.annotations.BackendData] == n.Annotations[ksm.annotations.BackendData] &&
		o.Annotations[ksm.annotations.BackendType] == n.Annotations[ksm.annotations.BackendType] &&
		o.Annotations[ksm.annotations.BackendPublicIP] == n.Annotations[ksm.annotations.BackendPublicIP] {
		changed = false
	}

	if ksm.enableIPv6 && o.Annotations[ksm.annotations.BackendV6Data] == n.Annotations[ksm.annotations.BackendV6Data] &&
		o.Annotations[ksm.annotations.BackendType] == n.Annotations[ksm.annotations.BackendType] &&
		o.Annotations[ksm.annotations.BackendPublicIPv6] == n.Annotations[ksm.annotations.BackendPublicIPv6] {
		changed = false
	}

	if !changed {
		return // No change to lease
	}

	l, err := ksm.nodeToLease(*n)
	if err != nil {
		log.Infof("Error turning node %q to lease: %v", n.ObjectMeta.Name, err)
		return
	}
	ksm.events <- subnet.Event{Type: subnet.EventAdded, Lease: l}
}

func (ksm *kubeSubnetManager) GetNetworkConfig(ctx context.Context) (*subnet.Config, error) {
	return ksm.subnetConf, nil
}

func (ksm *kubeSubnetManager) AcquireLease(ctx context.Context, attrs *subnet.LeaseAttrs) (*subnet.Lease, error) {
	cachedNode, err := ksm.nodeStore.Get(ksm.nodeName)
	if err != nil {
		return nil, err
	}

	n := cachedNode.DeepCopy()
	if n.Spec.PodCIDR == "" {
		return nil, fmt.Errorf("node %q pod cidr not assigned", ksm.nodeName)
	}

	var bd, v6Bd []byte
	bd, err = attrs.BackendData.MarshalJSON()
	if err != nil {
		return nil, err
	}

	v6Bd, err = attrs.BackendV6Data.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var cidr, ipv6Cidr *net.IPNet
	_, cidr, err = net.ParseCIDR(n.Spec.PodCIDR)
	if err != nil {
		return nil, err
	}

	for _, podCidr := range n.Spec.PodCIDRs {
		_, parseCidr, err := net.ParseCIDR(podCidr)
		if err != nil {
			return nil, err
		}
		if len(parseCidr.IP) == net.IPv6len {
			ipv6Cidr = parseCidr
			break
		}
	}

	anns := ksm.annotations

	var backendPublicIPOverwrite string
	var backendPublicIPOverwriteSource string

	if overwrite := n.Annotations[anns.BackendPublicIPOverwrite]; overwrite != "" {
		backendPublicIPOverwrite = overwrite
		backendPublicIPOverwriteSource = "annotation"
	} else if overwrite := n.Labels[anns.BackendPublicIPOverwrite]; overwrite != "" {
		backendPublicIPOverwrite = overwrite
		backendPublicIPOverwriteSource = "label"
	}

	var backendPublicIPv6Overwrite string
	var backendPublicIPv6OverwriteSource string

	if overwrite := n.Annotations[anns.BackendPublicIPv6Overwrite]; overwrite != "" {
		backendPublicIPv6Overwrite = overwrite
		backendPublicIPv6OverwriteSource = "annotation"
	} else if overwrite := n.Labels[anns.BackendPublicIPv6Overwrite]; overwrite != "" {
		backendPublicIPv6Overwrite = overwrite
		backendPublicIPv6OverwriteSource = "label"
	}

	if (n.Annotations[anns.BackendData] != string(bd) ||
		n.Annotations[anns.BackendType] != attrs.BackendType ||
		n.Annotations[anns.BackendPublicIP] != attrs.PublicIP.String() ||
		n.Annotations[anns.SubnetKubeManaged] != "true" ||
		(backendPublicIPOverwrite != "" && backendPublicIPOverwrite != attrs.PublicIP.String())) ||
		(attrs.PublicIPv6 != nil &&
			(n.Annotations[anns.BackendV6Data] != string(v6Bd) ||
				n.Annotations[anns.BackendType] != attrs.BackendType ||
				n.Annotations[anns.BackendPublicIPv6] != attrs.PublicIPv6.String() ||
				n.Annotations[anns.SubnetKubeManaged] != "true" ||
				(backendPublicIPv6Overwrite != "" && backendPublicIPv6Overwrite != attrs.PublicIPv6.String()))) {

		n.Annotations[anns.BackendType] = attrs.BackendType

		//TODO -i only vxlan and host-gw backends support dual stack now.
		if (attrs.BackendType == "vxlan" && string(bd) != "null") || (attrs.BackendType == "wireguard" && string(bd) != "null") || attrs.BackendType != "vxlan" {
			n.Annotations[anns.BackendData] = string(bd)
			if backendPublicIPOverwrite != "" {
				if n.Annotations[anns.BackendPublicIP] != backendPublicIPOverwrite {
					log.Infof("Overriding public ip with '%s' from node %s '%s'",
						backendPublicIPOverwrite,
						backendPublicIPOverwriteSource,
						anns.BackendPublicIPOverwrite)
					n.Annotations[anns.BackendPublicIP] = backendPublicIPOverwrite
				}
			} else {
				n.Annotations[anns.BackendPublicIP] = attrs.PublicIP.String()
			}
		}

		if (attrs.BackendType == "vxlan" && string(v6Bd) != "null") || (attrs.BackendType == "wireguard" && string(v6Bd) != "null" && attrs.PublicIPv6 != nil) || (attrs.BackendType == "host-gw" && attrs.PublicIPv6 != nil) {
			n.Annotations[anns.BackendV6Data] = string(v6Bd)
			if backendPublicIPv6Overwrite != "" {
				if n.Annotations[anns.BackendPublicIPv6] != backendPublicIPv6Overwrite {
					log.Infof("Overriding public ipv6 with '%s' from node %s '%s'",
						backendPublicIPv6Overwrite,
						backendPublicIPv6OverwriteSource,
						anns.BackendPublicIPv6Overwrite)
					n.Annotations[anns.BackendPublicIPv6] = backendPublicIPv6Overwrite
				}
			} else {
				n.Annotations[anns.BackendPublicIPv6] = attrs.PublicIPv6.String()
			}
		}
		n.Annotations[anns.SubnetKubeManaged] = "true"

		oldData, err := json.Marshal(cachedNode)
		if err != nil {
			return nil, err
		}

		newData, err := json.Marshal(n)
		if err != nil {
			return nil, err
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.Node{})
		if err != nil {
			return nil, fmt.Errorf("failed to create patch for node %q: %v", ksm.nodeName, err)
		}

		_, err = ksm.client.CoreV1().Nodes().Patch(ctx, ksm.nodeName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
		if err != nil {
			return nil, err
		}
	}
	if ksm.setNodeNetworkUnavailable {
		log.Infoln("Setting NodeNetworkUnavailable")
		err = ksm.setNodeNetworkUnavailableFalse(ctx)
		if err != nil {
			log.Errorf("Unable to set NodeNetworkUnavailable to False for %q: %v", ksm.nodeName, err)
		}
	} else {
		log.Infoln("Skip setting NodeNetworkUnavailable")
	}

	lease := &subnet.Lease{
		Attrs:      *attrs,
		Expiration: time.Now().Add(24 * time.Hour),
	}
	if cidr != nil && ksm.enableIPv4 {
		if !containsCIDR(ksm.subnetConf.Network.ToIPNet(), cidr) {
			return nil, fmt.Errorf("subnet %q specified in the flannel net config doesn't contain %q PodCIDR of the %q node.", ksm.subnetConf.Network, cidr, ksm.nodeName)
		}

		lease.Subnet = ip.FromIPNet(cidr)
	}
	if ipv6Cidr != nil {
		if ksm.subnetConf.IPv6Network.IP == nil {
			return nil, fmt.Errorf("subnet %q specified in the PodCIDR, but doesn't exist in the flannel net config of the %q node.", ipv6Cidr, ksm.nodeName)
		}

		if !containsCIDR(ksm.subnetConf.IPv6Network.ToIPNet(), ipv6Cidr) {
			return nil, fmt.Errorf("subnet %q specified in the flannel net config doesn't contain %q IPv6 PodCIDR of the %q node.", ksm.subnetConf.IPv6Network, ipv6Cidr, ksm.nodeName)
		}

		lease.IPv6Subnet = ip.FromIP6Net(ipv6Cidr)
	}
	//TODO - only vxlan, host-gw and wireguard backends support dual stack now.
	if attrs.BackendType != "vxlan" && attrs.BackendType != "host-gw" && attrs.BackendType != "wireguard" {
		lease.EnableIPv4 = true
		lease.EnableIPv6 = false
	}
	return lease, nil
}

func (ksm *kubeSubnetManager) WatchLeases(ctx context.Context, cursor interface{}) (subnet.LeaseWatchResult, error) {
	select {
	case event := <-ksm.events:
		return subnet.LeaseWatchResult{
			Events: []subnet.Event{event},
		}, nil
	case <-ctx.Done():
		return subnet.LeaseWatchResult{}, context.Canceled
	}
}

func (ksm *kubeSubnetManager) Run(ctx context.Context) {
	log.Infof("Starting kube subnet manager")
	ksm.nodeController.Run(ctx.Done())
}

func (ksm *kubeSubnetManager) nodeToLease(n v1.Node) (l subnet.Lease, err error) {
	if ksm.enableIPv4 {
		l.Attrs.PublicIP, err = ip.ParseIP4(n.Annotations[ksm.annotations.BackendPublicIP])
		if err != nil {
			return l, err
		}
		l.Attrs.BackendData = json.RawMessage(n.Annotations[ksm.annotations.BackendData])

		_, cidr, err := net.ParseCIDR(n.Spec.PodCIDR)
		if err != nil {
			return l, err
		}
		l.Subnet = ip.FromIPNet(cidr)
		l.EnableIPv4 = ksm.enableIPv4
	}

	if ksm.enableIPv6 {
		l.Attrs.PublicIPv6, err = ip.ParseIP6(n.Annotations[ksm.annotations.BackendPublicIPv6])
		if err != nil {
			return l, err
		}
		l.Attrs.BackendV6Data = json.RawMessage(n.Annotations[ksm.annotations.BackendV6Data])

		ipv6Cidr := new(net.IPNet)
		log.Infof("Creating the node lease for IPv6. This is the n.Spec.PodCIDRs: %v", n.Spec.PodCIDRs)
		for _, podCidr := range n.Spec.PodCIDRs {
			_, parseCidr, err := net.ParseCIDR(podCidr)
			if err != nil {
				return l, err
			}
			if len(parseCidr.IP) == net.IPv6len {
				ipv6Cidr = parseCidr
				break
			}
		}
		l.IPv6Subnet = ip.FromIP6Net(ipv6Cidr)
		l.EnableIPv6 = ksm.enableIPv6
	}
	l.Attrs.BackendType = n.Annotations[ksm.annotations.BackendType]
	return l, nil
}

// RenewLease: unimplemented
func (ksm *kubeSubnetManager) RenewLease(ctx context.Context, lease *subnet.Lease) error {
	return ErrUnimplemented
}

func (ksm *kubeSubnetManager) WatchLease(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, cursor interface{}) (subnet.LeaseWatchResult, error) {
	return subnet.LeaseWatchResult{}, ErrUnimplemented
}

func (ksm *kubeSubnetManager) Name() string {
	return fmt.Sprintf("Kubernetes Subnet Manager - %s", ksm.nodeName)
}

// Set Kubernetes NodeNetworkUnavailable to false when starting
// https://kubernetes.io/docs/concepts/architecture/nodes/#condition
func (ksm *kubeSubnetManager) setNodeNetworkUnavailableFalse(ctx context.Context) error {
	condition := v1.NodeCondition{
		Type:               v1.NodeNetworkUnavailable,
		Status:             v1.ConditionFalse,
		Reason:             "FlannelIsUp",
		Message:            "Flannel is running on this node",
		LastTransitionTime: metav1.Now(),
		LastHeartbeatTime:  metav1.Now(),
	}
	raw, err := json.Marshal(&[]v1.NodeCondition{condition})
	if err != nil {
		return err
	}
	patch := []byte(fmt.Sprintf(`{"status":{"conditions":%s}}`, raw))
	_, err = ksm.client.CoreV1().Nodes().PatchStatus(ctx, ksm.nodeName, patch)
	return err
}

func containsCIDR(ipnet1, ipnet2 *net.IPNet) bool {
	ones1, _ := ipnet1.Mask.Size()
	ones2, _ := ipnet2.Mask.Size()
	return ones1 <= ones2 && ipnet1.Contains(ipnet2.IP)
}
