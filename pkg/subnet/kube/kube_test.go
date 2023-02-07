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
	"net"

	"testing"
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
