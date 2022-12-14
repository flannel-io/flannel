// Copyright 2022 flannel authors
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
	"net"

	"github.com/flannel-io/flannel/pkg/subnet"
	"golang.org/x/net/context"
	networkingv1alpha1 "k8s.io/api/networking/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	log "k8s.io/klog"
)

// handleAddClusterCidr is called every time a clustercidr resource is added
// to the kubernetes cluster.
// In flanneld, we need to add the new CIDRs (IPv4 and/or IPv6) to the configuration
// and update the configuration file used by the flannel cni plugin.
func (ksm *kubeSubnetManager) handleAddClusterCidr(obj interface{}) {
	cluster := obj.(*networkingv1alpha1.ClusterCIDR)
	if cluster == nil {
		log.Errorf("received wrong object: %s", obj)
		return
	}
	if cluster.Spec.IPv4 != "" {
		log.Infof("handleAddClusterCidr: registering CIDR [ %s ]\n", cluster.Spec.IPv4)
		_, cidr, err := net.ParseCIDR(cluster.Spec.IPv4)
		if err != nil {
			log.Errorf("error reading cluster spec: %s", err)
			return
		}
		ksm.subnetConf.AddNetwork(cidr)
	}
	if cluster.Spec.IPv6 != "" {
		log.Infof("handleAddClusterCidr: registering CIDR [ %s ]\n", cluster.Spec.IPv6)
		_, cidr, err := net.ParseCIDR(cluster.Spec.IPv6)
		if err != nil {
			log.Errorf("error reading cluster spec: %s", err)
			return
		}
		ksm.subnetConf.AddNetwork(cidr)
	}

	err := subnet.WriteSubnetFile(ksm.snFileInfo.path, ksm.subnetConf, ksm.snFileInfo.ipMask, ksm.snFileInfo.sn, ksm.snFileInfo.IPv6sn, ksm.snFileInfo.mtu)
	if err != nil {
		log.Errorf("error writing subnet file: %s", err)
		return
	}
}

// handleDeleteClusterCidr is called when flannel is notified that a clustercidr resource was deleted in the cluster.
// Since this should not happen with the current API, we log an error.
func (ksm *kubeSubnetManager) handleDeleteClusterCidr(obj interface{}) {
	log.Error("deleting ClusterCIDR is not supported. This shouldn't get called")
}

// readFlannelNetworksFromClusterCIDRList calls the k8s API to read all the clustercidr resources
// that exists when flannel starts. The cidrs are used to populate the Networks and IPv6Networks
// entries in the flannel configuration.
// This function is only used once when flannel starts.
// Later, we rely on an Informer to keep the configuration updated.
func readFlannelNetworksFromClusterCIDRList(ctx context.Context, c clientset.Interface, sc *subnet.Config) error {
	clusters, err := c.NetworkingV1alpha1().ClusterCIDRs().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	log.Infof("reading %d ClusterCIDRs from kube api\n", len(clusters.Items))
	for _, item := range clusters.Items {
		if item.Spec.IPv4 != "" {
			_, cidr, err := net.ParseCIDR(item.Spec.IPv4)
			if err != nil {
				return err
			}
			log.Infof("adding IPv4 CIDR %s to config.Networks", cidr)
			sc.AddNetwork(cidr)
		}
		if item.Spec.IPv6 != "" {
			_, cidr, err := net.ParseCIDR((item.Spec.IPv6))
			if err != nil {
				return err
			}
			log.Infof("adding IPv6 CIDR %s to config.IPv6Networks", cidr)
			sc.AddNetwork(cidr)
		}
	}

	return nil
}
