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

package hostgw

import (
	"net"
	"runtime"
	"testing"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func TestRouteCache(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	origns, _ := netns.Get()
	defer origns.Close()
	newns, _ := netns.New()
	netns.Set(newns)
	defer newns.Close()

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
	nw := network{extIface: &backend.ExternalInterface{Iface: &net.Interface{Index: lo.Attrs().Index}}}
	gw1, gw2 := ip.FromIP(net.ParseIP("127.0.0.1")), ip.FromIP(net.ParseIP("127.0.0.2"))
	subnet1 := ip.IP4Net{IP: ip.FromIP(net.ParseIP("192.168.0.0")), PrefixLen: 24}
	nw.handleSubnetEvents([]subnet.Event{
		{Type: subnet.EventAdded, Lease: subnet.Lease{Subnet: subnet1, Attrs: subnet.LeaseAttrs{PublicIP: gw1, BackendType: "host-gw"}}},
	})
	if len(nw.rl) != 1 {
		t.Fatal(nw.rl)
	}
	if !routeEqual(nw.rl[0], netlink.Route{Dst: subnet1.ToIPNet(), Gw: gw1.ToIP()}) {
		t.Fatal(nw.rl[0])
	}
	// change gateway of previous route
	nw.handleSubnetEvents([]subnet.Event{
		{Type: subnet.EventAdded, Lease: subnet.Lease{
			Subnet: subnet1, Attrs: subnet.LeaseAttrs{PublicIP: gw2, BackendType: "host-gw"}}}})
	if len(nw.rl) != 1 {
		t.Fatal(nw.rl)
	}
	if !routeEqual(nw.rl[0], netlink.Route{Dst: subnet1.ToIPNet(), Gw: gw2.ToIP()}) {
		t.Fatal(nw.rl[0])
	}
}
