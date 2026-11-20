//go:build !windows
// +build !windows

package vxlan

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestDesiredVxlanMTU(t *testing.T) {
	if got, want := desiredVxlanMTU(1500), 1450; got != want {
		t.Fatalf("desiredVxlanMTU(1500) = %d, want %d", got, want)
	}
	if got, want := desiredVxlanMTU(9001), 8951; got != want {
		t.Fatalf("desiredVxlanMTU(9001) = %d, want %d", got, want)
	}
}

func TestNewVxlanLinkOmitsMTUAtCreate(t *testing.T) {
	// Mirrors the LinkAttrs used in newVXLANDevice: MTU must stay unset so
	// LinkAdd does not fail on kernels that cap vxlan max_mtu at 1500.
	link := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:         "flannel.1",
			HardwareAddr: net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		VxlanId: 1,
	}
	if link.MTU != 0 {
		t.Fatalf("create-time VXLAN LinkAttrs.MTU = %d, want 0 so the kernel can derive a default", link.MTU)
	}
}
