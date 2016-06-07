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

	syswin "golang.org/x/sys/windows"
)

type MibIfTable struct {
	NumEntries uint32
	Table      [0xffff]syswin.MibIfRow
}

type MibIpAddrTable struct {
	NumEntries uint32
	Table      [0xffff]MibIpAddrRow
}

const (
	MIB_IPADDR_PRIMARY      uint16 = 0x0001
	MIB_IPADDR_DYNAMIC      uint16 = 0x0004
	MIB_IPADDR_DISCONNECTED uint16 = 0x0008
	MIB_IPADDR_DELETED      uint16 = 0x0040
	MIB_IPADDR_TRANSIENT    uint16 = 0x0080
)

type MibIpAddrRow struct {
	Address           uint32
	Index             uint32
	Netmask           uint32
	BroadcastAddress  uint32
	MaxReassemblySize uint32
	Reserved          uint16
	AddressType       uint16
}

func GetIfTable(table *MibIfTable, size *uint32, ordered bool) (errcode error) {
	r0, _, _ := syscall.Syscall(procGetIfTable.Addr(), 3, uintptr(unsafe.Pointer(table)), uintptr(unsafe.Pointer(size)), uintptr(unsafe.Pointer(&ordered)))
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}

func GetIpAddrTable(table *MibIpAddrTable, size *uint32, ordered bool) (errcode error) {
	r0, _, _ := syscall.Syscall(procGetIpAddrTable.Addr(), 3, uintptr(unsafe.Pointer(table)), uintptr(unsafe.Pointer(size)), uintptr(unsafe.Pointer(&ordered)))
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}
