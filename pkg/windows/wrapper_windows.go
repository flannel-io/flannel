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

	"golang.org/x/sys/windows"
)

const (
	MAX_HOSTNAME_LEN = 128
	MAX_SCOPE_ID_LEN = 260
)

var (
	iphlp                    = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetNetworkParams     = iphlp.NewProc("GetNetworkParams")
	procGetIpForwardTable    = iphlp.NewProc("GetIpForwardTable")
	procCreateIpForwardEntry = iphlp.NewProc("CreateIpForwardEntry")
	procDeleteIpForwardEntry = iphlp.NewProc("DeleteIpForwardEntry")
	procGetIfTable           = iphlp.NewProc("GetIfTable")
	procGetIpAddrTable       = iphlp.NewProc("GetIpAddrTable")
)

type IPAddressString struct {
	String [16]byte
}

type IPAddrString struct {
	Next      *IPAddrString
	IPAddress IPAddressString
	IPMask    IPAddressString
	Context   uint32
}

type FixedInfo struct {
	Hostname         [MAX_HOSTNAME_LEN + 4]byte
	DomainName       [MAX_HOSTNAME_LEN + 4]byte
	CurrentDNSServer *IPAddrString
	DNSServerList    IPAddrString
	NodeType         uint
	ScopeID          [MAX_SCOPE_ID_LEN + 4]byte
	EnableRouting    bool
	EnableProxy      bool
	EnableDNS        bool
}

func GetNetworkParams(fixedInfo *FixedInfo, ol *uint32) (errcode error) {
	r0, _, _ := syscall.Syscall(procGetNetworkParams.Addr(), 2, uintptr(unsafe.Pointer(fixedInfo)), uintptr(unsafe.Pointer(ol)), 0)
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}
