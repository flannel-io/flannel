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
	"unsafe"

	"github.com/coreos/flannel/pkg/windows"
	syswin "golang.org/x/sys/windows"
)

// AddRoute adds a route to the system's routing table
func AddRoute(route *Route) error {
	return windows.CreateIpForwardEntry(routeToMibIPForwardRow(route))
}

// DeleteRoute deletes a route from the system's routing table
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
		}
	}

	return result, nil
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
