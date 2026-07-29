// Copyright 2024 flannel authors
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
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FuzzNodeToLease fuzzes the conversion of a Kubernetes Node into a flannel
// lease. The node's annotations and PodCIDR come from the Kubernetes API and
// are therefore untrusted; nodeToLease parses them with ParseIP4/ParseIP6,
// net.ParseCIDR and ip.FromIPNet/FromIP6Net, none of which should panic on
// malformed input.
func FuzzNodeToLease(f *testing.F) {
	ann, err := newAnnotations("flannel.alpha.coreos.com")
	if err != nil {
		f.Fatalf("failed to build annotations: %v", err)
	}

	// seeds: valid-ish and clearly malformed combinations.
	f.Add("192.168.0.1", "fc00::1", "10.244.1.0/24", "vxlan", true, false)
	f.Add("", "", "fc00::/64", "host-gw", false, true)
	f.Add("not-an-ip", "not-an-ip", "not-a-cidr", "", true, true)
	f.Add("192.168.0.1", "fc00::1", "fc00::/64", "vxlan", true, false)
	f.Add("::1", "192.168.0.1", "10.244.1.0/24", "wireguard", false, true)

	f.Fuzz(func(t *testing.T, publicIP, publicIPv6, podCIDR, backendType string, enableIPv4, enableIPv6 bool) {
		ksm := &kubeSubnetManager{
			enableIPv4:  enableIPv4,
			enableIPv6:  enableIPv6,
			annotations: ann,
			nodeName:    "fuzz-node",
		}

		node := v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					ann.BackendPublicIP:   publicIP,
					ann.BackendPublicIPv6: publicIPv6,
					ann.BackendType:       backendType,
				},
			},
			Spec: v1.NodeSpec{
				PodCIDR: podCIDR,
			},
		}

		// Only require that nodeToLease does not panic on arbitrary input;
		// returning an error for malformed data is expected behaviour.
		_, _ = ksm.nodeToLease(node)
	})
}
