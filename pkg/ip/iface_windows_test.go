//go:build windows
// +build windows

// Copyright 2017 flannel authors
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
	"testing"
)

func TestGetInterfaceIP4Addr(t *testing.T) {
	iface, err := GetDefaultGatewayInterface()
	if err != nil {
		t.Fatal(err)
	}

	_, err = GetInterfaceIP4Addrs(iface)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetDefaultGatewayInterface(t *testing.T) {
	_, err := GetDefaultGatewayInterface()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetInterfaceByIP(t *testing.T) {
	defaultIface, err := GetDefaultGatewayInterface() // use default gateway interface for test
	if err != nil {
		t.Fatal(err)
	}

	defaultIpv4Addr, err := GetInterfaceIP4Addrs(defaultIface)
	if err != nil {
		t.Fatal(err)
	}

	for _, addr := range defaultIpv4Addr {
		iface, err := GetInterfaceByIP(addr)
		if err != nil {
			t.Fatal(err)
		}

		if iface.Index != defaultIface.Index {
			t.Fatalf("iface.Index(%d) != defaultIface.Index(%d)", iface.Index, defaultIface.Index)
		}
	}
}

func TestEnableForwardingForInterface(t *testing.T) {
	iface, err := GetDefaultGatewayInterface() // use default gateway interface for test
	if err != nil {
		t.Fatal(err)
	}

	err = EnableForwardingForInterface(iface)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = DisableForwardingForInterface(iface) }() // try to reset forwarding

	enabled, err := IsForwardingEnabledForInterface(iface)
	if err != nil {
		t.Fatal(err)
	}

	if !enabled {
		t.Fatal("enabled == false")
	}
}
