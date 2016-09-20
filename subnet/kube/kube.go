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
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

var (
	ErrUnimplemented = errors.New("unimplemented")
	kubeSubnetCfg    *subnet.Config
)

const (
	resyncPeriod = 5 * time.Minute

	subnetKubeManagedAnnotation = "flannel.alpha.coreos.com/kube-subnet-manager"
	backendDataAnnotation       = "flannel.alpha.coreos.com/backend-data"
	backendTypeAnnotation       = "flannel.alpha.coreos.com/backend-type"
	backendPublicIPAnnotation   = "flannel.alpha.coreos.com/public-ip"

	netConfPath = "/etc/kube-flannel/net-conf.json"
)

type kubeSubnetManager struct {
	client         clientset.Interface
	nodeName       string
	nodeStore      cache.StoreToNodeLister
	nodeController *framework.Controller
	subnetConf     *subnet.Config
}

func NewSubnetManager() (subnet.Manager, error) {
	cfg, err := restclient.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to initialize inclusterconfig: %v", err)
	}
	c, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize client: %v", err)
	}

	podName := os.Getenv("POD_NAME")
	podNamespace := os.Getenv("POD_NAMESPACE")
	if podName == "" || podNamespace == "" {
		return nil, fmt.Errorf("env variables POD_NAME and POD_NAMESPACE must be set")
	}

	pod, err := c.Pods(podNamespace).Get(podName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving pod spec for '%s/%s': %v", podNamespace, podName, err)
	}
	nodeName := pod.Spec.NodeName
	if nodeName == "" {
		return nil, fmt.Errorf("node name not present in pod spec '%s/%s'", podNamespace, podName)
	}

	netConf, err := ioutil.ReadFile(netConfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read net conf: %v", err)
	}

	sc, err := subnet.ParseConfig(string(netConf))
	if err != nil {
		return nil, fmt.Errorf("error parsing subnet config: %s", err)
	}
	sm, err := newKubeSubnetManager(c, sc, nodeName)
	if err != nil {
		return nil, fmt.Errorf("error creating network manager: %s", err)
	}
	go sm.Run(context.Background())
	return sm, err
}

func newKubeSubnetManager(c clientset.Interface, sc *subnet.Config, nodeName string) (*kubeSubnetManager, error) {
	var ksm kubeSubnetManager
	ksm.client = c
	ksm.nodeName = nodeName
	ksm.subnetConf = sc
	ksm.nodeStore.Store, ksm.nodeController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return ksm.client.Core().Nodes().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return ksm.client.Core().Nodes().Watch(options)
			},
		},
		&api.Node{},
		resyncPeriod,
		framework.ResourceEventHandlerFuncs{},
	)
	return &ksm, nil
}

func (ksm *kubeSubnetManager) GetNetworkConfig(ctx context.Context, network string) (*subnet.Config, error) {
	return ksm.subnetConf, nil
}

func (ksm *kubeSubnetManager) AcquireLease(ctx context.Context, network string, attrs *subnet.LeaseAttrs) (*subnet.Lease, error) {
	nobj, found, err := ksm.nodeStore.Store.GetByKey(ksm.nodeName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("node %q not found", ksm.nodeName)
	}
	n, ok := nobj.(*api.Node)
	if !ok {
		return nil, fmt.Errorf("nobj was not a *api.Node")
	}
	if n.Spec.PodCIDR == "" {
		return nil, fmt.Errorf("node %q pod cidr not assigned", ksm.nodeName)
	}
	bd, err := attrs.BackendData.MarshalJSON()
	if err != nil {
		return nil, err
	}
	_, cidr, err := net.ParseCIDR(n.Spec.PodCIDR)
	if err != nil {
		return nil, err
	}
	if n.Annotations[backendDataAnnotation] != string(bd) ||
		n.Annotations[backendTypeAnnotation] != attrs.BackendType ||
		n.Annotations[backendPublicIPAnnotation] != attrs.PublicIP.String() ||
		n.Annotations[subnetKubeManagedAnnotation] != "true" {
		n.Annotations[backendTypeAnnotation] = attrs.BackendType
		n.Annotations[backendDataAnnotation] = string(bd)
		n.Annotations[backendPublicIPAnnotation] = attrs.PublicIP.String()
		n.Annotations[subnetKubeManagedAnnotation] = "true"
		n, err = ksm.client.Core().Nodes().Update(n)
		if err != nil {
			return nil, err
		}
	}
	return &subnet.Lease{
		Subnet:     ip.FromIPNet(cidr),
		Attrs:      *attrs,
		Expiration: time.Now().Add(24 * time.Hour),
	}, nil
}

func (ksm *kubeSubnetManager) RenewLease(ctx context.Context, network string, lease *subnet.Lease) error {
	return nil
}

func (ksm *kubeSubnetManager) WatchLease(ctx context.Context, network string, sn ip.IP4Net, cursor interface{}) (subnet.LeaseWatchResult, error) {
	time.Sleep(time.Second)
	nobj, found, err := ksm.nodeStore.Store.GetByKey(ksm.nodeName)
	if err != nil {
		return subnet.LeaseWatchResult{}, err
	}
	if !found {
		return subnet.LeaseWatchResult{}, fmt.Errorf("node %q not found", ksm.nodeName)
	}
	n, ok := nobj.(*api.Node)
	if !ok {
		return subnet.LeaseWatchResult{}, fmt.Errorf("nobj was not a *api.Node")
	}
	l, err := nodeToLease(*n)
	if err != nil {
		return subnet.LeaseWatchResult{}, err
	}
	return subnet.LeaseWatchResult{
		Snapshot: []subnet.Lease{l},
	}, nil
}

func (ksm *kubeSubnetManager) WatchLeases(ctx context.Context, network string, cursor interface{}) (subnet.LeaseWatchResult, error) {
	time.Sleep(time.Second)
	leases := make([]subnet.Lease, 0)
	nl, err := ksm.nodeStore.List()
	if err != nil {
		return subnet.LeaseWatchResult{}, err
	}
	for _, n := range nl.Items {
		if s, ok := n.Annotations[subnetKubeManagedAnnotation]; !ok || s != "true" {
			continue
		}
		l, err := nodeToLease(n)
		if err != nil {
			glog.Infof("error turning node %q to lease: %v", n.ObjectMeta.Name, err)
			continue
		}
		leases = append(leases, l)
	}
	return subnet.LeaseWatchResult{
		Snapshot: leases,
	}, nil
}

func (ksm *kubeSubnetManager) WatchNetworks(ctx context.Context, cursor interface{}) (subnet.NetworkWatchResult, error) {
	time.Sleep(time.Second)
	return subnet.NetworkWatchResult{
		Snapshot: []string{""},
	}, nil
}

func (ksm *kubeSubnetManager) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()
	glog.Infof("starting kube subnet manager")
	ksm.nodeController.Run(ctx.Done())
}

func nodeToLease(n api.Node) (l subnet.Lease, err error) {
	l.Attrs.PublicIP, err = ip.ParseIP4(n.Annotations[backendPublicIPAnnotation])
	if err != nil {
		return l, err
	}
	l.Attrs.BackendType = n.Annotations[backendTypeAnnotation]
	l.Attrs.BackendData = json.RawMessage(n.Annotations[backendDataAnnotation])
	_, cidr, err := net.ParseCIDR(n.Spec.PodCIDR)
	if err != nil {
		return l, err
	}
	l.Subnet = ip.FromIPNet(cidr)
	l.Expiration = time.Now().Add(24 * time.Hour)
	return l, nil
}

// unimplemented
func (ksm *kubeSubnetManager) RevokeLease(ctx context.Context, network string, sn ip.IP4Net) error {
	return ErrUnimplemented
}

func (ksm *kubeSubnetManager) AddReservation(ctx context.Context, network string, r *subnet.Reservation) error {
	return ErrUnimplemented
}

func (ksm *kubeSubnetManager) RemoveReservation(ctx context.Context, network string, subnet ip.IP4Net) error {
	return ErrUnimplemented
}

func (ksm *kubeSubnetManager) ListReservations(ctx context.Context, network string) ([]subnet.Reservation, error) {
	return nil, ErrUnimplemented
}
