//go:build !windows
// +build !windows

package ipmatch

import (
	"os/exec"
	"testing"
)

func TestLookupExtIface(t *testing.T) {
	var err error

	var execOrFail = func(name string, arg ...string) {
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
	execOrFail("sudo", "ip", "link", "set", "dummy0", "up")

	defer func() {
		exec.Command("sudo", "ip", "link", "set", "dummy0", "down").Run()
		exec.Command("sudo", "ip", "link", "delete", "dummy0").Run()
	}()

	t.Run("ByIfRegexForIPv4", func(t *testing.T) {
		backendInterface, err := LookupExtIface("", `192\.168\.200\.\d+`, IPv4Stack, PublicIPOpts{})
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
		backendInterface, err := LookupExtIface("", `dummy\d+`, IPv4Stack, PublicIPOpts{})
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
		backendInterface, err := LookupExtIface("dummy0", "", IPv4Stack, PublicIPOpts{})
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
		backendInterface, err := LookupExtIface("172.16.30.18", "", IPv4Stack, PublicIPOpts{})
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
		backendInterface, err := LookupExtIface("", `172\.16\.30\.\d+`, IPv4Stack, PublicIPOpts{
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
}
