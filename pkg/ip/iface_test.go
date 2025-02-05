//go:build !windows
// +build !windows

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
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestEnsureV4AddressOnLink(t *testing.T) {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatal(err)
	}
	// check changing address
	ipn := IP4Net{IP: FromIP(net.ParseIP("127.0.0.2")), PrefixLen: 24}
	if err := EnsureV4AddressOnLink(ipn, ipn, lo); err != nil {
		t.Fatal(err)
	}
	addrs, err := netlink.AddrList(lo, netlink.FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 || addrs[0].String() != "127.0.0.2/24 lo" {
		t.Fatalf("addrs %v is not expected", addrs)
	}

	// check changing address if there exist unknown addresses
	if err := netlink.AddrAdd(lo, &netlink.Addr{IPNet: &net.IPNet{IP: net.ParseIP("127.0.1.1"), Mask: net.CIDRMask(24, 32)}}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureV4AddressOnLink(ipn, ipn, lo); err != nil {
		t.Fatal(err)
	}
	addrs, err = netlink.AddrList(lo, netlink.FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 2 {
		t.Fatalf("two addresses expected, addrs: %v", addrs)
	}
}

func TestEnsureV6AddressOnLink(t *testing.T) {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatal(err)
	}
	// check changing address
	ipn := IP6Net{IP: FromIP6(net.ParseIP("::2")), PrefixLen: 64}
	if err := EnsureV6AddressOnLink(ipn, ipn, lo); err != nil {
		t.Fatal(err)
	}
	addrs, err := netlink.AddrList(lo, netlink.FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 || addrs[0].String() != "::2/64" {
		t.Fatalf("v6 addrs %v is not expected", addrs)
	}

	// check changing address if there exist multiple addresses
	if err := netlink.AddrAdd(lo, &netlink.Addr{IPNet: &net.IPNet{IP: net.ParseIP("2001::4"), Mask: net.CIDRMask(64, 128)}}); err != nil {
		t.Fatal(err)
	}
	addrs, err = netlink.AddrList(lo, netlink.FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 2 {
		t.Fatalf("two addresses expected, addrs: %v", addrs)
	}
	if err := EnsureV6AddressOnLink(ipn, ipn, lo); err != nil {
		t.Fatal(err)
	}
	addrs, err = netlink.AddrList(lo, netlink.FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 {
		t.Fatalf("only one address expected, addrs: %v", addrs)
	}
}
