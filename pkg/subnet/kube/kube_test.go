// Copyright 2018 flannel authors
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
	"context"
	"net"

	"testing"

	"github.com/flannel-io/flannel/pkg/lease"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestContainsCIDR(t *testing.T) {
	testCases := []struct {
		cidr1          string
		cidr2          string
		expectedResult bool
	}{
		{"10.244.0.0/16", "10.244.0.0/16", true},
		{"10.244.0.0/16", "10.244.0.0/24", true},
		{"10.244.0.0/16", "10.244.255.0/24", true},
		{"10.244.0.0/16", "10.244.0.0/15", false},
		{"10.244.0.0/16", "192.168.0.0/24", false},

		{"2001:0db8:1234::/48", "2001:0db8:1234::/48", true},
		{"2001:0db8:1234::/48", "2001:0db8:1234::/64", true},
		{"2001:0db8:1234::/48", "2001:0db8:1234:ffff::/64", true},
		{"2001:0db8:1234::/48", "2001:0db8:1234::/47", false},
		{"2001:0db8:1234::/48", "fe02::/32", false},
	}

	for i, tc := range testCases {
		_, ipnet1, _ := net.ParseCIDR(tc.cidr1)
		_, ipnet2, _ := net.ParseCIDR(tc.cidr2)

		actualResult := containsCIDR(ipnet1, ipnet2)

		if actualResult != tc.expectedResult {
			t.Errorf("#%d: Expected %t, but was %t.", i, tc.expectedResult, actualResult)
		}
	}
}

func TestHandleUpdateLeaseEvent(t *testing.T) {
	a, err := newAnnotations("flannel.alpha.coreos.com")
	if err != nil {
		t.Fatal(err)
	}

	newNode := func(changes map[string]string) *v1.Node {
		n := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
				Annotations: map[string]string{
					a.SubnetKubeManaged: "true",
					a.BackendType:       "vxlan",
					a.BackendData:       `{"VNI":1,"VtepMAC":"aa:bb:cc:dd:ee:01"}`,
					a.BackendPublicIP:   "192.0.2.10",
					a.BackendV6Data:     `{"VNI":1,"VtepMAC":"aa:bb:cc:dd:ee:02"}`,
					a.BackendPublicIPv6: "2001:db8::10",
				},
			},
			Spec: v1.NodeSpec{PodCIDRs: []string{"10.244.1.0/24", "2001:db8:1::/64"}},
		}
		for k, v := range changes {
			n.Annotations[k] = v
		}
		return n
	}

	testCases := []struct {
		name       string
		enableIPv4 bool
		enableIPv6 bool
		changes    map[string]string
		expected   bool
	}{
		{"dual-stack, nothing changed", true, true, nil, false},
		{"dual-stack, public ip", true, true, map[string]string{a.BackendPublicIP: "192.0.2.11"}, true},
		{"dual-stack, backend data", true, true, map[string]string{a.BackendData: `{"VNI":1,"VtepMAC":"aa:bb:cc:dd:ee:03"}`}, true},
		{"dual-stack, public ipv6", true, true, map[string]string{a.BackendPublicIPv6: "2001:db8::11"}, true},
		{"dual-stack, backend v6 data", true, true, map[string]string{a.BackendV6Data: `{"VNI":1,"VtepMAC":"aa:bb:cc:dd:ee:04"}`}, true},
		{"ipv4, public ip", true, false, map[string]string{a.BackendPublicIP: "192.0.2.11"}, true},
		{"ipv4, backend type", true, false, map[string]string{a.BackendType: "host-gw"}, true},
		{"ipv4, public ipv6", true, false, map[string]string{a.BackendPublicIPv6: "2001:db8::11"}, false},
		{"ipv6, public ipv6", false, true, map[string]string{a.BackendPublicIPv6: "2001:db8::11"}, true},
		{"ipv6, backend type", false, true, map[string]string{a.BackendType: "host-gw"}, true},
		{"ipv6, public ip", false, true, map[string]string{a.BackendPublicIP: "192.0.2.11"}, false},
	}

	for _, tc := range testCases {
		ksm := &kubeSubnetManager{
			enableIPv4:  tc.enableIPv4,
			enableIPv6:  tc.enableIPv6,
			annotations: a,
			events:      make(chan lease.Event, 1),
		}

		ksm.handleUpdateLeaseEvent(context.Background(), newNode(nil), newNode(tc.changes))

		if actual := len(ksm.events) == 1; actual != tc.expected {
			t.Errorf("%s: expected lease event %t, but was %t.", tc.name, tc.expected, actual)
		}
	}
}
