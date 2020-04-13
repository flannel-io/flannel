// +build !windows

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
// +build !windows

package ip

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"unsafe"

	"github.com/vishvananda/netlink"
)

const (
	siocEthtool = 0x8946 // linux/sockios.h

	// #define ETHTOOL_SRXCSUM		0x00000015 /* Set RX hw csum enable (ethtool_value) */
	ethtoolSRxCsum = 0x00000015 // linux/ethtool.h
	// #define ETHTOOL_STXCSUM		0x00000017 /* Set TX hw csum enable (ethtool_value) */
	ethtoolSTxCsum = 0x00000017 // linux/ethtool.h

	maxIfNameSize = 16 // linux/if.h
)

// linux/if.h 'struct ifreq'
type ifreq struct {
	Name [maxIfNameSize]byte
	Data uintptr
}

// linux/ethtool.h 'struct ethtool_value'
type ethtoolValue struct {
	Cmd  uint32
	Data uint32
}

// ethtool executes Linux ethtool syscall.
func ethtool(iface string, cmd, val uint32) (retval uint32, err error) {
	if len(iface)+1 > maxIfNameSize {
		return 0, fmt.Errorf("interface name is too long")
	}
	socket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return 0, err
	}
	defer syscall.Close(socket)

	// prepare ethtool request
	value := ethtoolValue{cmd, val}
	request := ifreq{Data: uintptr(unsafe.Pointer(&value))}
	copy(request.Name[:], iface)

	// ioctl system call
	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(socket), uintptr(siocEthtool),
		uintptr(unsafe.Pointer(&request)))
	if errno != 0 {
		return 0, errno
	}
	return value.Data, nil
}

// SetChecksumOffloading enables/disables Rx/Tx checksum offloading
// for the given interface.
func SetChecksumOffloading(ifName string, rxOn, txOn bool) error {
	var rxVal, txVal uint32
	if rxOn {
		rxVal = 1
	}
	if txOn {
		txVal = 1
	}
	_, err := ethtool(ifName, ethtoolSRxCsum, rxVal)
	if err != nil {
		return err
	}
	_, err = ethtool(ifName, ethtoolSTxCsum, txVal)
	return err
}

func getIfaceAddrs(iface *net.Interface) ([]netlink.Addr, error) {
	link := &netlink.Device{
		netlink.LinkAttrs{
			Index: iface.Index,
		},
	}

	return netlink.AddrList(link, syscall.AF_INET)
}

func GetIfaceIP4Addr(iface *net.Interface) (net.IP, error) {
	addrs, err := getIfaceAddrs(iface)
	if err != nil {
		return nil, err
	}

	// prefer non link-local addr
	var ll net.IP

	for _, addr := range addrs {
		if addr.IP.To4() == nil {
			continue
		}

		if addr.IP.IsGlobalUnicast() {
			return addr.IP, nil
		}

		if addr.IP.IsLinkLocalUnicast() {
			ll = addr.IP
		}
	}

	if ll != nil {
		// didn't find global but found link-local. it'll do.
		return ll, nil
	}

	return nil, errors.New("No IPv4 address found for given interface")
}

func GetIfaceIP4AddrMatch(iface *net.Interface, matchAddr net.IP) error {
	addrs, err := getIfaceAddrs(iface)
	if err != nil {
		return err
	}

	for _, addr := range addrs {
		// Attempt to parse the address in CIDR notation
		// and assert it is IPv4
		if addr.IP.To4() != nil {
			if addr.IP.To4().Equal(matchAddr) {
				return nil
			}
		}
	}

	return errors.New("No IPv4 address found for given interface")
}

func GetDefaultGatewayIface() (*net.Interface, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			if route.LinkIndex <= 0 {
				return nil, errors.New("Found default route but could not determine interface")
			}
			return net.InterfaceByIndex(route.LinkIndex)
		}
	}

	return nil, errors.New("Unable to find default route")
}

func GetInterfaceByIP(ip net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		err := GetIfaceIP4AddrMatch(&iface, ip)
		if err == nil {
			return &iface, nil
		}
	}

	return nil, errors.New("No interface with given IP found")
}

func DirectRouting(ip net.IP) (bool, error) {
	routes, err := netlink.RouteGet(ip)
	if err != nil {
		return false, fmt.Errorf("couldn't lookup route to %v: %v", ip, err)
	}
	if len(routes) == 1 && routes[0].Gw == nil {
		// There is only a single route and there's no gateway (i.e. it's directly connected)
		return true, nil
	}
	return false, nil
}

// EnsureV4AddressOnLink ensures that there is only one v4 Addr on `link` and it equals `ipn`.
// If there exist multiple addresses on link, it returns an error message to tell callers to remove additional address.
func EnsureV4AddressOnLink(ipn IP4Net, link netlink.Link) error {
	addr := netlink.Addr{IPNet: ipn.ToIPNet()}
	existingAddrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	// flannel will never make this happen. This situation can only be caused by a user, so get them to sort it out.
	if len(existingAddrs) > 1 {
		return fmt.Errorf("link has incompatible addresses. Remove additional addresses and try again. %#v", link)
	}

	// If the device has an incompatible address then delete it. This can happen if the lease changes for example.
	if len(existingAddrs) == 1 && !existingAddrs[0].Equal(addr) {
		if err := netlink.AddrDel(link, &existingAddrs[0]); err != nil {
			return fmt.Errorf("failed to remove IP address %s from %s: %s", ipn.String(), link.Attrs().Name, err)
		}
		existingAddrs = []netlink.Addr{}
	}

	// Actually add the desired address to the interface if needed.
	if len(existingAddrs) == 0 {
		if err := netlink.AddrAdd(link, &addr); err != nil {
			return fmt.Errorf("failed to add IP address %s to %s: %s", ipn.String(), link.Attrs().Name, err)
		}
	}

	return nil
}
