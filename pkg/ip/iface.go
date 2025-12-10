//go:build !windows
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

package ip

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	log "k8s.io/klog/v2"
)

func getIfaceAddrs(iface *net.Interface) ([]netlink.Addr, error) {
	link := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{
			Index: iface.Index,
		},
	}

	return netlink.AddrList(link, syscall.AF_INET)
}

func getIfaceV6Addrs(iface *net.Interface) ([]netlink.Addr, error) {
	link := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{
			Index: iface.Index,
		},
	}

	return netlink.AddrList(link, syscall.AF_INET6)
}

func GetInterfaceIP4Addrs(iface *net.Interface) ([]net.IP, error) {
	addrs, err := getIfaceAddrs(iface)
	if err != nil {
		return nil, err
	}

	// sort addresses in preferred usage order
	slices.SortFunc(addrs, compareAddrs)

	// map rich ip information from netlink into the stdlib data structure
	// while filtering addresses that aren't relevant or usable
	ipAddrs := make([]net.IP, 0)
	for _, i := range addrs {
		// address must be IPv4, global unicast or link-local unicast and non-deprecated
		if i.IP.To4() != nil &&
			(i.IP.IsGlobalUnicast() || i.IP.IsLinkLocalUnicast()) &&
			i.Flags&syscall.IFA_F_DEPRECATED == 0 {
			ipAddrs = append(ipAddrs, i.IP)
		}
	}

	if len(ipAddrs) > 0 {
		return ipAddrs, nil
	}

	return nil, errors.New("no IPv4 address found for given interface")
}

func GetInterfaceIP6Addrs(iface *net.Interface) ([]net.IP, error) {
	addrs, err := getIfaceV6Addrs(iface)
	if err != nil {
		return nil, err
	}

	// sort addresses in preferred usage order
	slices.SortFunc(addrs, compareAddrs)

	// map rich ip information from netlink into the stdlib data structure
	// while filtering addresses that aren't relevant or usable
	ipAddrs := make([]net.IP, 0)
	for _, i := range addrs {
		// address must be IPv6, global unicast or link-local unicast and non-deprecated
		if i.IP.To16() != nil &&
			(i.IP.IsGlobalUnicast() || i.IP.IsLinkLocalUnicast()) &&
			i.Flags&syscall.IFA_F_DEPRECATED == 0 {
			ipAddrs = append(ipAddrs, i.IP)
		}
	}

	if len(ipAddrs) > 0 {
		return ipAddrs, nil
	}

	return nil, errors.New("no IPv6 address found for given interface")
}

func GetInterfaceIP4AddrMatch(iface *net.Interface, matchAddr net.IP) error {
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

	return errors.New("no IPv4 address found for given interface")
}

func GetInterfaceIP6AddrMatch(iface *net.Interface, matchAddr net.IP) error {
	addrs, err := getIfaceV6Addrs(iface)
	if err != nil {
		return err
	}

	for _, addr := range addrs {
		// Attempt to parse the address in CIDR notation
		// and assert it is IPv6
		if addr.IP.To16() != nil {
			if addr.IP.To16().Equal(matchAddr) {
				return nil
			}
		}
	}

	return errors.New("no IPv6 address found for given interface")
}

func GetDefaultGatewayInterface() (*net.Interface, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			if route.LinkIndex <= 0 {
				return nil, errors.New("found default route but could not determine interface")
			}
			return net.InterfaceByIndex(route.LinkIndex)
		}
	}

	return nil, errors.New("unable to find default route")
}

func GetDefaultV6GatewayInterface() (*net.Interface, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET6)
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "::/0" {
			if route.LinkIndex <= 0 {
				return nil, errors.New("found default v6 route but could not determine interface")
			}
			return net.InterfaceByIndex(route.LinkIndex)
		}
	}

	return nil, errors.New("unable to find default v6 route")
}

func GetInterfaceByIP(ip net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		err := GetInterfaceIP4AddrMatch(&iface, ip)
		if err == nil {
			return &iface, nil
		}
	}

	return nil, errors.New("no interface with given IP found")
}

func GetInterfaceByIP6(ip net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		err := GetInterfaceIP6AddrMatch(&iface, ip)
		if err == nil {
			return &iface, nil
		}
	}

	return nil, errors.New("no interface with given IPv6 found")
}

func GetInterfaceBySpecificIPRouting(ip net.IP) (*net.Interface, net.IP, error) {
	routes, err := netlink.RouteGet(ip)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't lookup route to %v: %v", ip, err)
	}

	for _, route := range routes {
		iface, err := net.InterfaceByIndex(route.LinkIndex)
		if err != nil {
			return nil, nil, fmt.Errorf("couldn't lookup interface: %v", err)
		} else {
			return iface, route.Src, nil
		}
	}

	return nil, nil, errors.New("no interface with given IP found")
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

// EnsureV4AddressOnLink ensures that there is only one v4 Addr on `link` within the `ipn` address space and it equals `ipa`.
func EnsureV4AddressOnLink(ipa IP4Net, ipn IP4Net, link netlink.Link) error {
	addr := netlink.Addr{IPNet: ipa.ToIPNet()}
	existingAddrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	var hasAddr bool
	for _, existingAddr := range existingAddrs {
		if existingAddr.Equal(addr) {
			hasAddr = true
			continue
		}

		if ipn.Contains(FromIP(existingAddr.IP)) {
			if err := netlink.AddrDel(link, &existingAddr); err != nil {
				return fmt.Errorf("failed to remove IP address %s from %s: %s", existingAddr.String(), link.Attrs().Name, err)
			}
			log.Infof("removed IP address %s from %s", existingAddr.String(), link.Attrs().Name)
		}
	}

	// Actually add the desired address to the interface if needed.
	if !hasAddr {
		if err := netlink.AddrAdd(link, &addr); err != nil {
			return fmt.Errorf("failed to add IP address %s to %s: %s", addr.String(), link.Attrs().Name, err)
		}
	}

	return nil
}

// EnsureV6AddressOnLink ensures that there is only one v6 Addr on `link` and it equals `ipn`.
// If there exist multiple addresses on link, it returns an error message to tell callers to remove additional address.
func EnsureV6AddressOnLink(ipa IP6Net, ipn IP6Net, link netlink.Link) error {
	addr := netlink.Addr{IPNet: ipa.ToIPNet()}
	existingAddrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		return err
	}

	onlyLinkLocal := true
	for _, existingAddr := range existingAddrs {
		if !existingAddr.IP.IsLinkLocalUnicast() {
			if !existingAddr.Equal(addr) {
				if err := netlink.AddrDel(link, &existingAddr); err != nil {
					return fmt.Errorf("failed to remove v6 IP address %s from %s: %w", ipn.String(), link.Attrs().Name, err)
				}
				existingAddrs = []netlink.Addr{}
				onlyLinkLocal = false
			} else {
				return nil
			}
		}
	}

	if onlyLinkLocal {
		existingAddrs = []netlink.Addr{}
	}

	// Actually add the desired address to the interface if needed.
	if len(existingAddrs) == 0 {
		if err := netlink.AddrAdd(link, &addr); err != nil {
			return fmt.Errorf("failed to add v6 IP address %s to %s: %w", ipn.String(), link.Attrs().Name, err)
		}
	}

	return nil
}

func AddBlackholeV4Route(ipV4Dest *net.IPNet) error {
	route := netlink.Route{Dst: ipV4Dest, Type: unix.RTN_BLACKHOLE, Family: netlink.FAMILY_V4}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &route, netlink.RT_FILTER_DST|netlink.RT_FILTER_TYPE)
	if err != nil && len(routes) == 0 {
		err = netlink.RouteAdd(&route)
	}
	return err
}

func AddBlackholeV6Route(ipV6Dest *net.IPNet) error {
	route := netlink.Route{Dst: ipV6Dest, Type: unix.RTN_BLACKHOLE, Family: netlink.FAMILY_V6}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V6, &route, netlink.RT_FILTER_DST|netlink.RT_FILTER_TYPE)
	if err != nil && len(routes) == 0 {
		err = netlink.RouteAdd(&route)
	}
	return err
}

// compareAddrs compares two netlink address in terms of which one should be used preferably.
// Addresses which are supposed to be completely disregarded are not considered specially and should be filtered separately
func compareAddrs(a, b netlink.Addr) int {
	// global unicast addresses are preferable to link-local addresses because they probably have better reachability
	if a.IP.IsGlobalUnicast() && b.IP.IsLinkLocalUnicast() {
		return -1
	} else if a.IP.IsLinkLocalUnicast() && b.IP.IsGlobalUnicast() {
		return 1
	}

	// manually assigned addresses are preferable to auto-assigned or generated addresses
	if a.Flags&syscall.IFA_F_PERMANENT == syscall.IFA_F_PERMANENT && b.Flags&syscall.IFA_F_PERMANENT == 0 {
		return -1
	} else if a.Flags&syscall.IFA_F_PERMANENT == 0 && b.Flags&syscall.IFA_F_PERMANENT == syscall.IFA_F_PERMANENT {
		return 1
	}

	// non-temporary address are preferable to temporary addresses
	if a.Flags&syscall.IFA_F_TEMPORARY == 0 && b.Flags&syscall.IFA_F_TEMPORARY == syscall.IFA_F_TEMPORARY {
		return -1
	} else if a.Flags&syscall.IFA_F_TEMPORARY == syscall.IFA_F_TEMPORARY && b.Flags&syscall.IFA_F_TEMPORARY == 0 {
		return 1
	}

	// anything else doesn't really matter
	return 0
}
