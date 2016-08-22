// +build windows

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
	"net"
	"reflect"
	"syscall"
	"unsafe"

	"github.com/coreos/flannel/pkg/windows"
	syswin "golang.org/x/sys/windows"
)

//This mimics the exact values from github.com/vishvananda/netlink/nl/nl_linux.go
const (
	FAMILY_ALL = syscall.AF_UNSPEC
	FAMILY_V4  = syscall.AF_INET
	FAMILY_V6  = syscall.AF_INET6
)

const (
	RT_FILTER_PROTOCOL uint64 = 1 << (1 + iota)
	RT_FILTER_SCOPE
	RT_FILTER_TYPE
	RT_FILTER_TOS
	RT_FILTER_IIF
	RT_FILTER_OIF
	RT_FILTER_DST
	RT_FILTER_SRC
	RT_FILTER_GW
	RT_FILTER_TABLE
)

// AddRoute adds a route to the system's routing table
func AddRoute(route *Route) error {
	return windows.CreateIpForwardEntry(routeToMibIPForwardRow(route))
}

// RouteDel deletes a route from the system's routing table
func DeleteRoute(route *Route) error {
	return windows.DeleteIpForwardEntry(routeToMibIPForwardRow(route))
}

// RouteList returns a list of routes
func RouteList() ([]Route, error) {
	var t windows.MibIPForwardTable
	b := make([]byte, reflect.TypeOf(t).Size())
	l := uint32(len(b))
	a := (*windows.MibIPForwardTable)(unsafe.Pointer(&b[0]))

	err := windows.GetIpForwardTable(a, &l, true)
	if err == syswin.ERROR_INSUFFICIENT_BUFFER {
		b = make([]byte, l)
		err = windows.GetIpForwardTable(a, &l, true)
	}

	result := make([]Route, int(a.NumEntries))
	for i := 0; i < int(a.NumEntries); i++ {
		result[i] = Route{
			Destination: &net.IPNet{
				IP:   windows.InetNtoa(a.Table[i].ForwardDest),
				Mask: windows.ToIPMask(a.Table[i].ForwardMask),
			},
			Gateway:   windows.InetNtoa(a.Table[i].ForwardNextHop),
			LinkIndex: int(a.Table[i].ForwardIfIndex),
			Type:      int(a.Table[i].ForwardType),
		}
	}

	return result, nil
}

func RouteListFiltered(family int, filter *Route, filterMask uint64) ([]Route, error) {
	//in windows we only use dst as filter as this is only
	//used for checking for existing routes in hostgw config
	routes, err := RouteList()
	var res []Route
	if err != nil {
		return nil, err
	}
	for _, route := range routes {
		if filter != nil {

			if filter.Destination != nil {
				if route.Destination == nil {
					continue
				}
				aMaskLen, aMaskBits := route.Destination.Mask.Size()
				bMaskLen, bMaskBits := filter.Destination.Mask.Size()

				if !(filter.Destination.IP.Equal(route.Destination.IP) &&
					aMaskLen == bMaskLen && aMaskBits == bMaskBits) {
					continue
				}
				res = append(res, route)
			}
		}
	}

	return res, nil
}

func routeToMibIPForwardRow(route *Route) *windows.MibIPForwardRow {
	return &windows.MibIPForwardRow{
		ForwardDest:    windows.NetIPToDWORD(route.Destination.IP),
		ForwardMask:    windows.NetIPMaskToDWORD(route.Destination.Mask),
		ForwardProto:   windows.MIB_IPPROTO_NETMGMT,
		ForwardType:    windows.MIB_IPROUTE_TYPE_DIRECT,
		ForwardMetric1: 10,
		ForwardIfIndex: uint32(route.LinkIndex),
		ForwardNextHop: windows.NetIPToDWORD(route.Gateway),
	}
}
