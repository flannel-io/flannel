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

package windows

import (
	"syscall"
	"unsafe"
)

const (
	MIB_IPPROTO_OTHER       = uint32(1)
	MIB_IPPROTO_LOCAL       = uint32(2)
	MIB_IPPROTO_NETMGMT     = uint32(3)
	MIB_IPPROTO_ICMP        = uint32(4)
	MIB_IPPROTO_EGP         = uint32(5)
	MIB_IPPROTO_GGP         = uint32(6)
	MIB_IPPROTO_HELLO       = uint32(7)
	MIB_IPPROTO_RIP         = uint32(8)
	MIB_IPPROTO_IS_IS       = uint32(9)
	MIB_IPPROTO_ES_IS       = uint32(10)
	MIB_IPPROTO_CISCO       = uint32(11)
	MIB_IPPROTO_BBN         = uint32(12)
	MIB_IPPROTO_OSPF        = uint32(13)
	MIB_IPPROTO_BGP         = uint32(14)
	MIB_IPPROTO_IPDR        = uint32(15)
	MIB_IPPROTO_EIGRP       = uint32(16)
	MIB_IPPROTO_DVMRP       = uint32(17)
	MIB_IPPROTO_RPL         = uint32(18)
	MIB_IPPROTO_DHCP        = uint32(19)
	MIB_IPNT_AUTOSTATIC     = uint32(10002)
	MIB_IPNT_STATIC         = uint32(10006)
	MIB_IPNT_STATIC_NON_DOD = uint32(10007)

	MIB_IPROUTE_TYPE_OTHER    = uint32(1)
	MIB_IPROUTE_TYPE_INVALID  = uint32(2)
	MIB_IPROUTE_TYPE_DIRECT   = uint32(3)
	MIB_IPROUTE_TYPE_INDIRECT = uint32(4)
)

type MibIPForwardRow struct {
	ForwardDest      uint32
	ForwardMask      uint32
	ForwardPolicy    uint32
	ForwardNextHop   uint32
	ForwardIfIndex   uint32
	ForwardType      uint32
	ForwardProto     uint32
	ForwardAge       uint32
	ForwardNextHopAS uint32
	ForwardMetric1   uint32
	ForwardMetric2   uint32
	ForwardMetric3   uint32
	ForwardMetric4   uint32
	ForwardMetric5   uint32
}

type MibIPForwardTable struct {
	NumEntries uint32
	Table      [0xffff]MibIPForwardRow
}

func GetIpForwardTable(table *MibIPForwardTable, size *uint32, ordered bool) (errcode error) {
	r0, _, _ := syscall.Syscall(procGetIpForwardTable.Addr(), 3, uintptr(unsafe.Pointer(table)), uintptr(unsafe.Pointer(size)), uintptr(unsafe.Pointer(&ordered)))
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}

func CreateIpForwardEntry(route *MibIPForwardRow) (errcode error) {
	r0, _, _ := syscall.Syscall(procCreateIpForwardEntry.Addr(), 1, uintptr(unsafe.Pointer(route)), 0, 0)
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}

func DeleteIpForwardEntry(route *MibIPForwardRow) (errcode error) {
	r0, _, _ := syscall.Syscall(procDeleteIpForwardEntry.Addr(), 1, uintptr(unsafe.Pointer(route)), 0, 0)
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}
