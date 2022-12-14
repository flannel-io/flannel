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
//go:build !windows
// +build !windows

package backend

import (
	"net"
	"testing"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/ns"
	"github.com/flannel-io/flannel/pkg/subnet"
	"github.com/vishvananda/netlink"
)

func TestRouteCache(t *testing.T) {
	teardown := ns.SetUpNetlinkTest(t)
	defer teardown()

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.AddrAdd(lo, &netlink.Addr{IPNet: &net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(32, 32)}}); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatal(err)
	}
	nw := RouteNetwork{
		SimpleNetwork: SimpleNetwork{
			ExtIface: &ExternalInterface{Iface: &net.Interface{Index: lo.Attrs().Index}},
		},
		BackendType: "host-gw",
		LinkIndex:   lo.Attrs().Index,
	}
	nw.GetRoute = func(lease *subnet.Lease) *netlink.Route {
		return &netlink.Route{
			Dst:       lease.Subnet.ToIPNet(),
			Gw:        lease.Attrs.PublicIP.ToIP(),
			LinkIndex: nw.LinkIndex,
		}
	}
	gw1, gw2 := ip.FromIP(net.ParseIP("127.0.0.1")), ip.FromIP(net.ParseIP("127.0.0.2"))
	subnet1 := ip.IP4Net{IP: ip.FromIP(net.ParseIP("192.168.0.0")), PrefixLen: 24}
	nw.handleSubnetEvents([]subnet.Event{
		{Type: subnet.EventAdded, Lease: subnet.Lease{
			Subnet: subnet1, EnableIPv4: true, Attrs: subnet.LeaseAttrs{PublicIP: gw1, BackendType: "host-gw"}}},
	})
	if len(nw.routes) != 1 {
		t.Fatal(nw.routes)
	}
	if !routeEqual(nw.routes[0], netlink.Route{Dst: subnet1.ToIPNet(), Gw: gw1.ToIP(), LinkIndex: lo.Attrs().Index}) {
		t.Fatal(nw.routes[0])
	}
	// change gateway of previous route
	nw.handleSubnetEvents([]subnet.Event{
		{Type: subnet.EventAdded, Lease: subnet.Lease{
			Subnet: subnet1, EnableIPv4: true, Attrs: subnet.LeaseAttrs{PublicIP: gw2, BackendType: "host-gw"}}}})
	if len(nw.routes) != 1 {
		t.Fatal(nw.routes)
	}
	if !routeEqual(nw.routes[0], netlink.Route{Dst: subnet1.ToIPNet(), Gw: gw2.ToIP(), LinkIndex: lo.Attrs().Index}) {
		t.Fatal(nw.routes[0])
	}
}

func TestV6RouteCache(t *testing.T) {
	teardown := ns.SetUpNetlinkTest(t)
	defer teardown()

	la := netlink.NewLinkAttrs()
	la.Name = "br"
	br := &netlink.Bridge{LinkAttrs: la}
	if err := netlink.LinkAdd(br); err != nil {
		t.Fatal(err)
	}
	if err := netlink.AddrAdd(br, &netlink.Addr{IPNet: &net.IPNet{IP: net.ParseIP("2001:db8:1::1"), Mask: net.CIDRMask(64, 128)}}); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(br); err != nil {
		t.Fatal(err)
	}

	nw := RouteNetwork{
		SimpleNetwork: SimpleNetwork{
			ExtIface: &ExternalInterface{Iface: &net.Interface{Index: br.Attrs().Index}},
		},
		BackendType: "host-gw",
		LinkIndex:   br.Attrs().Index,
	}
	nw.GetV6Route = func(lease *subnet.Lease) *netlink.Route {
		return &netlink.Route{
			Dst:       lease.IPv6Subnet.ToIPNet(),
			Gw:        lease.Attrs.PublicIPv6.ToIP(),
			LinkIndex: nw.LinkIndex,
		}
	}
	gw1, gw2 := ip.FromIP6(net.ParseIP("2001:db8:1::2")), ip.FromIP6(net.ParseIP("2001:db8:1::10"))
	subnet1 := ip.IP6Net{IP: ip.FromIP6(net.ParseIP("2001:db8:ffff::")), PrefixLen: 64}
	nw.handleSubnetEvents([]subnet.Event{
		{Type: subnet.EventAdded, Lease: subnet.Lease{
			IPv6Subnet: subnet1, EnableIPv6: true, Attrs: subnet.LeaseAttrs{PublicIPv6: gw1, BackendType: "host-gw"}}},
	})
	if len(nw.v6Routes) != 1 {
		t.Fatal(nw.v6Routes)
	}
	if !routeEqual(nw.v6Routes[0], netlink.Route{Dst: subnet1.ToIPNet(), Gw: gw1.ToIP(), LinkIndex: br.Attrs().Index}) {
		t.Fatal(nw.v6Routes[0])
	}
	// change gateway of previous route
	nw.handleSubnetEvents([]subnet.Event{
		{Type: subnet.EventAdded, Lease: subnet.Lease{
			IPv6Subnet: subnet1, EnableIPv6: true, Attrs: subnet.LeaseAttrs{PublicIPv6: gw2, BackendType: "host-gw"}}}})
	linkbr, _ := netlink.LinkByName("br")
	routes, _ := netlink.RouteList(linkbr, 6)
	IsGw := ""
	for _, route := range routes {
		if len(route.Gw) != 0 {
			IsGw = route.Gw.String()
		}
	}

	if IsGw != gw2.String() {
		t.Fatal("Expected Gateway: ", gw2, " is not the same as the configured gateway: ", IsGw)
	}

	if len(nw.v6Routes) != 1 {
		t.Fatal(nw.v6Routes)
	}
	if !routeEqual(nw.v6Routes[0], netlink.Route{Dst: subnet1.ToIPNet(), Gw: gw2.ToIP(), LinkIndex: br.Attrs().Index}) {
		t.Fatal(nw.v6Routes[0])
	}
}
