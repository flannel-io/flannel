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
// +build !windows

package backend

import (
	"net"
	"testing"

	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/pkg/ns"
	"github.com/coreos/flannel/subnet"
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
			Subnet: subnet1, Attrs: subnet.LeaseAttrs{PublicIP: gw1, BackendType: "host-gw"}}},
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
			Subnet: subnet1, Attrs: subnet.LeaseAttrs{PublicIP: gw2, BackendType: "host-gw"}}}})
	if len(nw.routes) != 1 {
		t.Fatal(nw.routes)
	}
	if !routeEqual(nw.routes[0], netlink.Route{Dst: subnet1.ToIPNet(), Gw: gw2.ToIP(), LinkIndex: lo.Attrs().Index}) {
		t.Fatal(nw.routes[0])
	}
}
