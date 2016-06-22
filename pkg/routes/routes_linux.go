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

import (
	"github.com/vishvananda/netlink"
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
