// Copyright 2015 flannel authors
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

package ip

import (
	"encoding/json"
	"net"
	"testing"
)

func mkIP4Net(s string, plen uint) IP4Net {
	ip, err := ParseIP4(s)
	if err != nil {
		panic(err)
	}
	return IP4Net{ip, plen}
}

func mkIP4(s string) IP4 {
	ip, err := ParseIP4(s)
	if err != nil {
		panic(err)
	}
	return ip
}

func TestIP4(t *testing.T) {
	nip := net.ParseIP("1.2.3.4")
	ip := FromIP(nip)
	a, b, c, d := ip.Octets()
	if a != 1 || b != 2 || c != 3 || d != 4 {
		t.Error("FromIP failed")
	}

	ip, err := ParseIP4("1.2.3.4")
	if err != nil {
		t.Error("ParseIP4 failed with: ", err)
	} else {
		a, b, c, d := ip.Octets()
		if a != 1 || b != 2 || c != 3 || d != 4 {
			t.Error("ParseIP4 failed")
		}
	}

	if ip.ToIP().String() != "1.2.3.4" {
		t.Error("ToIP failed")
	}

	if ip.String() != "1.2.3.4" {
		t.Error("String failed")
	}

	if ip.StringSep("*") != "1*2*3*4" {
		t.Error("StringSep failed")
	}

	j, err := json.Marshal(ip)
	if err != nil {
		t.Error("Marshal of IP4 failed: ", err)
	} else if string(j) != `"1.2.3.4"` {
		t.Error("Marshal of IP4 failed with unexpected value: ", j)
	}

	addresses := []*struct {
		ip      string
		private bool
	}{
		{"192.168.0.1", true},
		{"172.16.0.1", true},
		{"172.31.0.1", true},
		{"10.1.2.3", true},

		{"8.8.8.8", false},
		{"172.32.0.1", false},
		{"192.167.0.1", false},
		{"192.169.0.1", false},
	}

	for _, address := range addresses {
		ip := mkIP4(address.ip)
		is_private := ip.IsPrivate()
		if is_private != address.private {
			t.Errorf("%v misdetected expected private: %v got private: %v", address.ip, address.private, is_private)
		}
	}
}

func TestIP4Net(t *testing.T) {
	n1 := mkIP4Net("1.2.3.0", 24)

	if n1.ToIPNet().String() != "1.2.3.0/24" {
		t.Error("ToIPNet failed")
	}

	if !n1.Overlaps(n1) {
		t.Errorf("%s does not overlap %s", n1, n1)
	}

	n2 := mkIP4Net("1.2.0.0", 16)
	if !n1.Overlaps(n2) {
		t.Errorf("%s does not overlap %s", n1, n2)
	}

	n2 = mkIP4Net("1.2.4.0", 24)
	if n1.Overlaps(n2) {
		t.Errorf("%s overlaps %s", n1, n2)
	}

	n2 = mkIP4Net("7.2.4.0", 22)
	if n1.Overlaps(n2) {
		t.Errorf("%s overlaps %s", n1, n2)
	}

	if !n1.Contains(mkIP4("1.2.3.0")) {
		t.Error("Contains failed")
	}

	if !n1.Contains(mkIP4("1.2.3.4")) {
		t.Error("Contains failed")
	}

	if n1.Contains(mkIP4("1.2.4.0")) {
		t.Error("Contains failed")
	}

	j, err := json.Marshal(n1)
	if err != nil {
		t.Error("Marshal of IP4Net failed: ", err)
	} else if string(j) != `"1.2.3.0/24"` {
		t.Error("Marshal of IP4Net failed with unexpected value: ", j)
	}

	n1.IncrementIP()
	if n1.String() != "1.2.3.1/24" {
		t.Error("IncrementIP() failed")
	}
}
