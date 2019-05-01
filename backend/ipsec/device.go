// +build !windows

// Copyright 2019 flannel authors
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

package ipsec

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"

	"github.com/coreos/flannel/subnet"
)

func readGw() (net.IP, int, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: nil}, netlink.RT_FILTER_DST)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to read routing table: %v", err)
	}
	if len(routes) != 1 {
		return nil, 0, fmt.Errorf("No default route could be determined")
	}
	defaultGw := routes[0].Gw
	defaultLink := routes[0].LinkIndex
	return defaultGw, defaultLink, nil
}

func buildRoute(myLease *subnet.Lease, remoteLease *subnet.Lease) (*netlink.Route, error) {
	// Manually build the routes to all remote leases against the IPSec routing table (220)
	// To ensure packages coming from the CNI interface will go correctly into the IPSec tunnel.
	gw, link, err := readGw()
	if err != nil {
		return nil, err
	}
	// Increment the network address by one to get the cni0 interface address
	netIP := myLease.Subnet.IP.ToIP()
	netIP[len(netIP)-1]++

	return &netlink.Route{
		Table:    220, // use the ipsec table
		Protocol: 4,   // proto static
		Dst: &net.IPNet{ // local CNI -> remoteLease
			IP:   remoteLease.Subnet.IP.ToIP(),
			Mask: net.CIDRMask(int(remoteLease.Subnet.PrefixLen), 32),
		},
		Gw:        gw,
		Src:       netIP,
		LinkIndex: link,
	}, nil
}

func (n *network) AddRoute(lease *subnet.Lease) error {
	r, err := buildRoute(n.Lease(), lease)
	if err != nil {
		return err
	}
	return netlink.RouteAdd(r)
}

func (n *network) DelRoute(lease *subnet.Lease) error {
	r, err := buildRoute(n.Lease(), lease)
	if err != nil {
		return err
	}
	return netlink.RouteDel(r)
}
