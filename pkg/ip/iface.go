package ip

import (
	"errors"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
)

func GetIfaceIP4Addr(iface *net.Interface) (net.IP, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	// prefer non link-local addr
	var ll net.IP

	for _, addr := range addrs {
		// Attempt to parse the address in CIDR notation
		// and assert it is IPv4
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil || ip.To4() == nil {
			continue
		}

		if ip.IsGlobalUnicast() {
			return ip, nil
		}

		if ip.IsLinkLocalUnicast() {
			ll = ip
		}
	}

	if ll != nil {
		// didn't find global but found link-local. it'll do.
		return ll, nil
	}

	return nil, errors.New("No IPv4 address found for given interface")
}

func GetIfaceIP4AddrMatch(iface *net.Interface, matchAddr net.IP) error {
	addrs, err := iface.Addrs()
	if err != nil {
		return err
	}

	for _, addr := range addrs {
		// Attempt to parse the address in CIDR notation
		// and assert it is IPv4
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.To4() != nil {
			if ip.To4().Equal(matchAddr) {
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
