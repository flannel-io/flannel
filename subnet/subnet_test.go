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

package subnet

import (
	"github.com/flannel-io/flannel/pkg/ip"
	"testing"
)

func TestSubnetNodev4(t *testing.T) {
	key := "10.12.13.0-24"
	sn, sn6 := ParseSubnetKey(key)

	if sn == nil {
		t.Errorf("Failed to parse ipv4 address")
		return
	}

	if sn.ToIPNet() == nil {
		t.Errorf("Failed to transform sn into IPNet")
		return
	}

	if sn.ToIPNet().String() != "10.12.13.0/24" {
		t.Errorf("Unexpected ipv4 network")
	}

	if sn6 != nil {
		t.Errorf("Not expecting ipv6 address")
	}

	if MakeSubnetKey(*sn, ip.IP6Net{}) != key {
		t.Errorf("MakeSubnetKey doesn't match parsed key")
	}
}

func TestSubnetNodev6(t *testing.T) {
	key := "10.12.13.0-24&fd00:12:13::-56"
	sn, sn6 := ParseSubnetKey(key)

	if sn == nil {
		t.Errorf("Failed to parse ipv4 address")
		return
	}

	if sn.ToIPNet() == nil {
		t.Errorf("Failed to transform sn into IPNet")
		return
	}

	if sn.ToIPNet().String() != "10.12.13.0/24" {
		t.Errorf("Unexpected ipv4 network")
	}

	if sn6 == nil {
		t.Errorf("Failed to parse ipv6 address")
		return
	}

	if sn6.ToIPNet().String() != "fd00:12:13::/56" {
		t.Errorf("Unexpected ipv6 network")
	}

	if MakeSubnetKey(*sn, *sn6) != key {
		t.Errorf("MakeSubnetKey doesn't match parsed key")
	}
}

func TestSubnetNodeInvalid(t *testing.T) {
	keys := []string{
		"10",
		"10.12.13.0",
		"10.12.13-24",
		"10.12.13.300-24",
		"10.12.13.0-24hi",
		"&2001::-56",
		"10.12.13.0-24&:12:13:-56",
		"10.12.13.0-24&20011::-56",
		"10.12.13.0-24&2001-56",
		"10.12.13.0-24&2001::",
		"10.12.13.0-24&2001::-56hi",
	}
	for _, key := range keys {
		sn, sn6 := ParseSubnetKey(key)

		if sn != nil || sn6 != nil {
			t.Errorf("Unexpectedly parsed %v - %v %v", key, sn, sn6)
		}
	}

}
