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

package ip

import (
	"fmt"
	"net"
	"reflect"
	"unsafe"

	"github.com/coreos/flannel/pkg/windows"
	syswin "golang.org/x/sys/windows"
)

func GetIfaceIP4Addr(iface *net.Interface) (net.IP, error) {
	var t windows.MibIpAddrTable
	b := make([]byte, reflect.TypeOf(t).Size())
	l := uint32(len(b))
	a := (*windows.MibIpAddrTable)(unsafe.Pointer(&b[0]))

	err := windows.GetIpAddrTable(a, &l, true)
	if err == syswin.ERROR_INSUFFICIENT_BUFFER {
		b = make([]byte, l)
		err = windows.GetIpAddrTable(a, &l, true)
	}

	for i := uint32(0); i < a.NumEntries; i++ {
		if int(a.Table[i].Index) == iface.Index && a.Table[i].AddressType&windows.MIB_IPADDR_PRIMARY == 1 {
			return windows.InetNtoa(a.Table[i].Address), nil
		}
	}

	return nil, fmt.Errorf("GetIfaceIP4Addr: Could not find primary address for interface")
}

func GetDefaultGatewayIface() (*net.Interface, error) {
	iftableMap := map[uint32]*syswin.MibIfRow{}
	iftable, err := getInterfaces()
	if err != nil {
		return nil, err
	}

	for i := uint32(0); i < iftable.NumEntries; i++ {
		iftableMap[iftable.Table[i].Index] = &iftable.Table[i]
	}

	var t windows.MibIPForwardTable
	b := make([]byte, reflect.TypeOf(t).Size())
	l := uint32(len(b))
	a := (*windows.MibIPForwardTable)(unsafe.Pointer(&b[0]))

	err = windows.GetIpForwardTable(a, &l, true)
	if err == syswin.ERROR_INSUFFICIENT_BUFFER {
		b = make([]byte, l)
		err = windows.GetIpForwardTable(a, &l, true)
	}

	var result net.Interface
	var i uint32
	for i = 0; i < a.NumEntries; i++ {
		entry := a.Table[i]

		if windows.InetNtoa(entry.ForwardDest).String() == "0.0.0.0" {
			iface, ok := iftableMap[entry.ForwardIfIndex]
			if !ok {
				continue
			}

			result = net.Interface{
				Index:        int(entry.ForwardIfIndex),
				MTU:          int(iface.Mtu),
				Name:         syswin.UTF16ToString(iface.Name[:]),
				HardwareAddr: iface.PhysAddr[:iface.PhysAddrLen],
				Flags:        net.FlagUp,
			}
		}
	}

	return &result, nil
}

func GetInterfaceByIP(ip net.IP) (*net.Interface, error) {
	var t windows.MibIpAddrTable
	b := make([]byte, reflect.TypeOf(t).Size())
	l := uint32(len(b))
	a := (*windows.MibIpAddrTable)(unsafe.Pointer(&b[0]))

	err := windows.GetIpAddrTable(a, &l, true)
	if err == syswin.ERROR_INSUFFICIENT_BUFFER {
		b = make([]byte, l)
		err = windows.GetIpAddrTable(a, &l, true)
	}

	iftableMap := map[uint32]*syswin.MibIfRow{}
	iftable, err := getInterfaces()
	if err != nil {
		return nil, err
	}

	for i := uint32(0); i < iftable.NumEntries; i++ {
		iftableMap[iftable.Table[i].Index] = &iftable.Table[i]
	}

	for i := uint32(0); i < a.NumEntries; i++ {
		if windows.InetNtoa(a.Table[i].Address).Equal(ip) {
			winIface := iftableMap[a.Table[i].Index]

			iface := net.Interface{
				Index:        int(a.Table[i].Index),
				MTU:          int(winIface.Mtu),
				Name:         syswin.UTF16ToString(winIface.Name[:]),
				HardwareAddr: winIface.PhysAddr[:winIface.PhysAddrLen],
				Flags:        net.FlagUp,
			}
			return &iface, nil
		}
	}

	return nil, fmt.Errorf("GetIfaceIP4Addr: Could not find primary address for interface")
}

func getInterfaces() (table *windows.MibIfTable, errcode error) {
	var t windows.MibIfTable
	b := make([]byte, reflect.TypeOf(t).Size())
	l := uint32(len(b))
	a := (*windows.MibIfTable)(unsafe.Pointer(&b[0]))

	err := windows.GetIfTable(a, &l, true)
	return a, err
}
