//go:build !windows
// +build !windows

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

package ipmatch

import (
	"os/exec"
	"testing"
)

func TestLookupExtIface(t *testing.T) {
	var err error

	execOrFail := func(name string, arg ...string) {
		err = exec.Command(name, arg...).Run()
		if err != nil {
			t.Fatalf("exec ip link command failed.\nmaybe you need adjust sudo config to allow user exec root command without input password.\nerr=%v", err)
		}
	}
	execOrFail("sudo", "ip", "link", "add", "name", "dummy0", "type", "dummy")
	execOrFail("sudo", "ip", "addr", "add", "1.10.100.1", "dev", "dummy0")
	execOrFail("sudo", "ip", "addr", "add", "192.168.200.128", "dev", "dummy0")
	execOrFail("sudo", "ip", "addr", "add", "172.16.30.18", "dev", "dummy0")
	execOrFail("sudo", "ip", "addr", "add", "172.16.31.200", "dev", "dummy0")
	execOrFail("sudo", "ip", "addr", "add", "172.16.32.100", "dev", "dummy0")
	execOrFail("sudo", "ip", "link", "set", "dummy0", "up")
	execOrFail("sudo", "ip", "route", "add", "172.16.32.254", "via", "172.16.32.100", "dev", "dummy0")

	defer func() {
		_ = exec.Command("sudo", "ip", "link", "set", "dummy0", "down").Run()
		_ = exec.Command("sudo", "ip", "link", "delete", "dummy0").Run()
	}()

	t.Run("ByIfRegexForIPv4", func(t *testing.T) {
		backendInterface, err := LookupExtIface("", `192\.168\.200\.\d+`, "", ipv4Stack, PublicIPOpts{})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("backendInterface=%+v iface=%+v", backendInterface, *backendInterface.Iface)

		if backendInterface.Iface.Name != "dummy0" {
			t.Fatalf("iface name not equal, expected=%v actual=%v", "dummy0", backendInterface.Iface.Name)
		}
		if backendInterface.IfaceAddr.String() != "192.168.200.128" {
			t.Fatalf("iface addr not equal, expected=%v actual=%v", "192.168.200.128", backendInterface.IfaceAddr.String())
		}
	})

	t.Run("ByIfRegexForName", func(t *testing.T) {
		backendInterface, err := LookupExtIface("", `dummy\d+`, "", ipv4Stack, PublicIPOpts{})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("backendInterface=%+v iface=%+v", backendInterface, *backendInterface.Iface)

		if backendInterface.Iface.Name != "dummy0" {
			t.Fatalf("iface name not equal, expected=%v actual=%v", "dummy0", backendInterface.Iface.Name)
		}
		if backendInterface.IfaceAddr.String() != "1.10.100.1" {
			t.Fatalf("iface addr not equal, expected=%v actual=%v", "1.10.100.1", backendInterface.IfaceAddr.String())
		}
	})

	t.Run("ByName", func(t *testing.T) {
		backendInterface, err := LookupExtIface("dummy0", "", "", ipv4Stack, PublicIPOpts{})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("backendInterface=%+v iface=%+v", backendInterface, *backendInterface.Iface)

		if backendInterface.Iface.Name != "dummy0" {
			t.Fatalf("iface name not equal, expected=%v actual=%v", "dummy0", backendInterface.Iface.Name)
		}
		if backendInterface.IfaceAddr.String() != "1.10.100.1" {
			t.Fatalf("iface addr not equal, expected=%v actual=%v", "1.10.100.1", backendInterface.IfaceAddr.String())
		}
	})

	t.Run("ByIPv4", func(t *testing.T) {
		backendInterface, err := LookupExtIface("172.16.30.18", "", "", ipv4Stack, PublicIPOpts{})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("backendInterface=%+v iface=%+v", backendInterface, *backendInterface.Iface)

		if backendInterface.Iface.Name != "dummy0" {
			t.Fatalf("iface name not equal, expected=%v actual=%v", "dummy0", backendInterface.Iface.Name)
		}
		if backendInterface.IfaceAddr.String() != "172.16.30.18" {
			t.Fatalf("iface addr not equal, expected=%v actual=%v", "172.16.30.18", backendInterface.IfaceAddr.String())
		}
	})

	t.Run("ByIfRegexMatchPublicIPv4", func(t *testing.T) {
		expectedIfaceName := "dummy0"
		expectedIP := "172.16.30.18"
		expectedPublicIP := "172.16.31.200"
		backendInterface, err := LookupExtIface("", `172\.16\.30\.\d+`, "", ipv4Stack, PublicIPOpts{
			PublicIP: expectedPublicIP,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("backendInterface=%+v iface=%+v", backendInterface, *backendInterface.Iface)

		if backendInterface.Iface.Name != "dummy0" {
			t.Fatalf("iface name not equal, expected=%v actual=%v", expectedIfaceName, backendInterface.Iface.Name)
		}
		if backendInterface.IfaceAddr.String() != expectedIP {
			t.Fatalf("iface addr not equal, expected=%v actual=%v", expectedIP, backendInterface.IfaceAddr.String())
		}
		if backendInterface.ExtAddr.String() != expectedPublicIP {
			t.Fatalf("iface addr not equal, expected=%v actual=%v", expectedPublicIP, backendInterface.ExtAddr.String())
		}
	})

	t.Run("ByIfCanReach", func(t *testing.T) {
		expectedIfaceName := "dummy0"
		expectedIP := "172.16.32.100"
		backendInterface, err := LookupExtIface("", "", "172.16.32.254", ipv4Stack, PublicIPOpts{})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("backendInterface=%+v iface=%+v", backendInterface, *backendInterface.Iface)

		if backendInterface.Iface.Name != expectedIfaceName {
			t.Fatalf("iface name not equal, expected=%v actual=%v", expectedIfaceName, backendInterface.Iface.Name)
		}
		if backendInterface.IfaceAddr.String() != expectedIP {
			t.Fatalf("iface addr not equal, expected=%v actual=%v", expectedIP, backendInterface.IfaceAddr.String())
		}
	})
}
