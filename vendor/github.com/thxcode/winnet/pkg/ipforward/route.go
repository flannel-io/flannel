package ipforward

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/thxcode/winnet/pkg/converters"
	"github.com/thxcode/winnet/pkg/syscalls"
)

const (
	DefaultRouteMetric int = 256
)

type Route struct {
	LinkIndex         int
	DestinationSubnet *net.IPNet
	GatewayAddress    net.IP
	RouteMetric       int
}

func (r Route) String() string {
	return fmt.Sprintf("link: %d, destnation: %s, gateway: %s, metric: %d",
		r.LinkIndex,
		r.DestinationSubnet.String(),
		r.GatewayAddress.String(),
		r.RouteMetric,
	)
}

func (r *Route) Equal(route Route) bool {
	if r.DestinationSubnet.IP.Equal(route.DestinationSubnet.IP) && r.GatewayAddress.Equal(route.GatewayAddress) && bytes.Equal(r.DestinationSubnet.Mask, route.DestinationSubnet.Mask) {
		return true
	}

	return false
}

// GetNetRoutesAll returns all nets routes on the host
func GetNetRoutesAll() (routes []Route, err error) {
	defer func() {
		if r := recover(); r != nil {
			if err == nil {
				err = fmt.Errorf("panic while getting all nets routes: %v", r)
			}
		}
	}()

	// find out how big our buffer needs to be
	b := make([]byte, 1)
	ft := (*syscalls.MibIpForwardTable)(unsafe.Pointer(&b[0]))
	ol := uint32(0)
	syscalls.GetIpForwardTable(ft, &ol, false)

	// start to get table
	b = make([]byte, ol)
	ft = (*syscalls.MibIpForwardTable)(unsafe.Pointer(&b[0]))
	if err := syscalls.GetIpForwardTable(ft, &ol, false); err != nil {
		return nil, fmt.Errorf("failed to get all nets routes: could not call system GetAdaptersInfo: %v", err)
	}

	// iterate to find
	for i := 0; i < int(ft.NumEntries); i++ {
		fr := *(*syscalls.MibIpForwardRow)(unsafe.Pointer(
			uintptr(unsafe.Pointer(&ft.Table[0])) + uintptr(i)*uintptr(unsafe.Sizeof(ft.Table[0])), // head idx + offset
		))

		frIfIndex := int(fr.ForwardIfIndex)
		frDestinationAddress := converters.Inet_ntoa(fr.ForwardDest, false)
		frDestinationMask := converters.Inet_ntoa(fr.ForwardMask, false)
		frDestinationIpNet := parseStringToIpNet(frDestinationAddress, frDestinationMask)
		frNextHop := converters.Inet_ntoa(fr.ForwardNextHop, false)
		frMetric := int(fr.ForwardMetric1)

		routes = append(routes, Route{
			LinkIndex:         frIfIndex,
			DestinationSubnet: frDestinationIpNet,
			GatewayAddress:    net.ParseIP(frNextHop),
			RouteMetric:       frMetric,
		})
	}

	return routes, nil
}

// GetNetRoutes returns nets routes by link and destination subnet
func GetNetRoutes(linkIndex int, destinationSubnet *net.IPNet) (routes []Route, err error) {
	defer func() {
		if r := recover(); r != nil {
			if err == nil {
				err = fmt.Errorf("panic while getting nets routes: %v", r)
			}
		}
	}()

	// find out how big our buffer needs to be
	b := make([]byte, 1)
	ft := (*syscalls.MibIpForwardTable)(unsafe.Pointer(&b[0]))
	ol := uint32(0)
	syscalls.GetIpForwardTable(ft, &ol, false)

	// start to get table
	b = make([]byte, ol)
	ft = (*syscalls.MibIpForwardTable)(unsafe.Pointer(&b[0]))
	if err := syscalls.GetIpForwardTable(ft, &ol, false); err != nil {
		return nil, fmt.Errorf("failed to get nets routes: could not call system GetAdaptersInfo: %v", err)
	}

	// iterate to find
	for i := 0; i < int(ft.NumEntries); i++ {
		fr := *(*syscalls.MibIpForwardRow)(unsafe.Pointer(
			uintptr(unsafe.Pointer(&ft.Table[0])) + uintptr(i)*uintptr(unsafe.Sizeof(ft.Table[0])), // head idx + offset
		))

		frIfIndex := int(fr.ForwardIfIndex)
		if frIfIndex != linkIndex {
			continue
		}

		frDestinationAddress := converters.Inet_ntoa(fr.ForwardDest, false)
		frDestinationMask := converters.Inet_ntoa(fr.ForwardMask, false)
		frDestinationIpNet := parseStringToIpNet(frDestinationAddress, frDestinationMask)
		if !isIpNetsEqual(frDestinationIpNet, destinationSubnet) {
			continue
		}

		frNextHop := converters.Inet_ntoa(fr.ForwardNextHop, false)
		frMetric := int(fr.ForwardMetric1)

		routes = append(routes, Route{
			LinkIndex:         frIfIndex,
			DestinationSubnet: frDestinationIpNet,
			GatewayAddress:    net.ParseIP(frNextHop),
			RouteMetric:       frMetric,
		})
	}

	return routes, nil
}

// NewNetRoute creates a new routes
func NewNetRoute(linkIndex int, destinationSubnet *net.IPNet, gatewayAddress net.IP) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if err == nil {
				err = fmt.Errorf("panic while creating new routes: %v", r)
			}
		}
	}()

	ifRow := &syscalls.MibIpInterfaceRow{
		Family:         syscall.AF_INET, // IPv4
		InterfaceIndex: uint32(linkIndex),
	}
	if err := syscalls.GetIpInterfaceEntry(ifRow); err != nil {
		return fmt.Errorf("could not get interface %d: %v", linkIndex, err)
	}

	frDestinationAddress := converters.Inet_aton(destinationSubnet.IP.String(), false)
	frDestinationMaskIp := net.ParseIP("255.255.255.255").Mask(destinationSubnet.Mask)
	frDestinationMask := converters.Inet_aton(frDestinationMaskIp.String(), false)
	frNextHop := converters.Inet_aton(gatewayAddress.String(), false)

	fr := &syscalls.MibIpForwardRow{
		ForwardDest:      frDestinationAddress,
		ForwardMask:      frDestinationMask,
		ForwardPolicy:    uint32(0),
		ForwardNextHop:   frNextHop,
		ForwardIfIndex:   uint32(linkIndex),
		ForwardType:      uint32(0), // MIB_IPROUTE_TYPE_DIRECT: A local routes where the next hop is the final destination (a local interface).
		ForwardProto:     uint32(3), // MIB_IPPROTO_NETMGMT: A static routes.
		ForwardAge:       uint32(0),
		ForwardNextHopAS: uint32(0),
		ForwardMetric1:   ifRow.Metric + uint32(DefaultRouteMetric),
		ForwardMetric2:   uint32(0),
		ForwardMetric3:   uint32(0),
		ForwardMetric4:   uint32(0),
		ForwardMetric5:   uint32(0),
	}

	if err := syscalls.CreateIpForwardEntry(fr); err != nil {
		return fmt.Errorf("failed to create nets routes: %v", err)
	}

	return nil
}

// RemoveNetRoute removes an existing routes
func RemoveNetRoute(linkIndex int, destinationSubnet *net.IPNet, gatewayAddress net.IP) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if err == nil {
				err = fmt.Errorf("panic while deleting nets routes: %v", r)
			}
		}
	}()

	// find out how big our buffer needs to be
	b := make([]byte, 1)
	ft := (*syscalls.MibIpForwardTable)(unsafe.Pointer(&b[0]))
	ol := uint32(0)
	syscalls.GetIpForwardTable(ft, &ol, false)

	// start to get table
	b = make([]byte, ol)
	ft = (*syscalls.MibIpForwardTable)(unsafe.Pointer(&b[0]))
	if err := syscalls.GetIpForwardTable(ft, &ol, false); err != nil {
		return fmt.Errorf("failed to remove nets routes: could not call system GetAdaptersInfo: %v", err)
	}

	// iterate to find
	for i := 0; i < int(ft.NumEntries); i++ {
		fr := *(*syscalls.MibIpForwardRow)(unsafe.Pointer(
			uintptr(unsafe.Pointer(&ft.Table[0])) + uintptr(i)*uintptr(unsafe.Sizeof(ft.Table[0])), // head idx + offset
		))

		frIfIndex := int(fr.ForwardIfIndex)
		if frIfIndex != linkIndex {
			continue
		}

		frDestinationAddress := converters.Inet_ntoa(fr.ForwardDest, false)
		frDestinationMask := converters.Inet_ntoa(fr.ForwardMask, false)
		frDestinationIpNet := parseStringToIpNet(frDestinationAddress, frDestinationMask)
		if !isIpNetsEqual(frDestinationIpNet, destinationSubnet) {
			continue
		}

		frNextHop := converters.Inet_ntoa(fr.ForwardNextHop, false)
		if frNextHop != gatewayAddress.String() {
			continue
		}

		if err := syscalls.DeleteIpForwardEntry(&fr); err != nil {
			return fmt.Errorf("failed to delete nets routes: %v", err)
		}

		return nil
	}

	return fmt.Errorf("there isn't a nets routes dest %s gw %s on interface %d", destinationSubnet, gatewayAddress, linkIndex)
}

func parseStringToIpNet(addr, mask string) *net.IPNet {
	ipNet := &net.IPNet{
		IP:   net.ParseIP(addr),
		Mask: make(net.IPMask, net.IPv4len),
	}
	for i, mask := range strings.SplitN(mask, ".", 4) {
		aInt, _ := strconv.Atoi(mask)
		aIntByte := byte(aInt)
		ipNet.Mask[i] = aIntByte
	}

	ipNet.IP = ipNet.IP.Mask(ipNet.Mask)
	return ipNet
}

func isIpNetsEqual(left, right *net.IPNet) bool {
	return left.IP.Equal(right.IP) && bytes.Equal(left.Mask, right.Mask)
}
