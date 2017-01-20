// +build linux

// Copyright 2016 flannel authors
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

package routes

import "github.com/vishvananda/netlink"

//This mimics the exact values from github.com/vishvananda/netlink/nl/nl_linux.go
const (
	FAMILY_ALL = netlink.FAMILY_ALL
	FAMILY_V4  = netlink.FAMILY_V4
	FAMILY_V6  = netlink.FAMILY_V6
)

const (
	RT_FILTER_PROTOCOL = netlink.RT_FILTER_PROTOCOL
	RT_FILTER_SCOPE    = netlink.RT_FILTER_SCOPE
	RT_FILTER_TYPE     = netlink.RT_FILTER_TYPE
	RT_FILTER_TOS      = netlink.RT_FILTER_TOS
	RT_FILTER_IIF      = netlink.RT_FILTER_IIF
	RT_FILTER_OIF      = netlink.RT_FILTER_OIF
	RT_FILTER_DST      = netlink.RT_FILTER_DST
	RT_FILTER_SRC      = netlink.RT_FILTER_SRC
	RT_FILTER_GW       = netlink.RT_FILTER_GW
	RT_FILTER_TABLE    = netlink.RT_FILTER_TABLE
)

// AddRoute adds a route to the system's routing table
func AddRoute(route *Route) error {
	r := netlink.Route{
		Dst:       route.Destination,
		Gw:        route.Gateway,
		LinkIndex: route.LinkIndex,
	}
	return netlink.RouteAdd(&r)
}

// DeleteRoute deletes a route from the system's routing table
func DeleteRoute(route *Route) error {
	r := netlink.Route{
		Dst:       route.Destination,
		Gw:        route.Gateway,
		LinkIndex: route.LinkIndex,
	}
	return netlink.RouteDel(&r)
}

// RouteList returns a list of routes
func RouteList() ([]Route, error) {
	rl, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}

	result := make([]Route, len(rl))
	for i, r := range rl {
		result[i] = Route{
			Destination: r.Dst,
			Gateway:     r.Gw,
			LinkIndex:   r.LinkIndex,
		}
	}

	return result, nil
}

//RouteListFiltered returns a list of routes filtered by
//the parameters passed to the function
func RouteListFiltered(family int, filter *Route, filterMask uint64) ([]Route, error) {
	nr := netlink.Route{
		Dst:       filter.Destination,
		Gw:        filter.Gateway,
		LinkIndex: filter.LinkIndex,
	}
	routes, err := netlink.RouteListFiltered(family, &nr, filterMask)
	if err != nil {
		return nil, err
	}

	result := make([]Route, len(routes))

	for i, r := range routes {
		result[i] = Route{
			Destination: r.Dst,
			Gateway:     r.Gw,
			LinkIndex:   r.LinkIndex,
		}
	}
	return result, nil
}
