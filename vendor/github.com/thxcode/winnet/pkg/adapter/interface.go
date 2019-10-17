package adapter

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/thxcode/winnet/pkg/converters"
	"github.com/thxcode/winnet/pkg/syscalls"
	"golang.org/x/sys/windows"
)

type Ipv4Interface struct {
	Idx                   int
	Name                  string
	Description           string
	IpAddress             string
	IpMask                string
	DefaultGatewayAddress string
	DhcpEnabled           bool
}

func (i Ipv4Interface) String() string {
	return fmt.Sprintf("name: %s, description: %s, index: %d, dhcpEnabled: %v, address: %s, mask: %s, gateway: %s",
		i.Name,
		i.Description,
		i.Idx,
		i.DhcpEnabled,
		i.IpAddress,
		i.IpMask,
		i.DefaultGatewayAddress,
	)
}

func (i *Ipv4Interface) GetSubnet() *net.IPNet {
	subnetIpNet := &net.IPNet{
		IP:   net.ParseIP(i.IpAddress),
		Mask: make(net.IPMask, net.IPv4len),
	}
	for i, m := range strings.SplitN(i.IpMask, ".", 4) {
		aInt, _ := strconv.Atoi(m)
		aIntByte := byte(aInt)
		subnetIpNet.IP[12+i] &= aIntByte
		subnetIpNet.Mask[i] = aIntByte
	}

	return subnetIpNet
}

// GetDefaultGatewayIfaceName get the interface name that has the default gateway
func GetDefaultGatewayIfaceName() (string, error) {
	ifaceIdx, err := GetDefaultGatewayIfaceIndex()
	if err != nil {
		return "", fmt.Errorf("failed to get default gateway interface: %v", err)
	}

	iface, err := GetInterfaceByIndex(ifaceIdx)
	if err != nil {
		return "", fmt.Errorf("failed to get default gateway interface by index(%d): %v", ifaceIdx, err)
	}

	return iface.Name, nil
}

// GetInterfaceByName gets an interface by name
func GetInterfaceByName(name string) (Ipv4Interface, error) {
	ifaces, err := GetInterfaces()
	if err != nil {
		return Ipv4Interface{}, err
	}

	for _, iface := range ifaces {
		if iface.Name == name {
			return iface, nil
		}
	}

	return Ipv4Interface{}, fmt.Errorf("could not get interface by name: %v", name)
}

// GetInterfaceByIP gets an interface by ip address in the format a.b.c.d
func GetInterfaceByIP(ipAddr string) (Ipv4Interface, error) {
	ifaces, err := GetInterfaces()
	if err != nil {
		return Ipv4Interface{}, err
	}

	for _, iface := range ifaces {
		if iface.IpAddress == ipAddr {
			return iface, nil
		}
	}

	return Ipv4Interface{}, fmt.Errorf("could not get interface by ip: %v", ipAddr)
}

// GetInterfaceByIndex gets an interface by index
func GetInterfaceByIndex(idx int) (Ipv4Interface, error) {
	ifaces, err := GetInterfaces()
	if err != nil {
		return Ipv4Interface{}, err
	}

	for _, iface := range ifaces {
		if iface.Idx == idx {
			return iface, nil
		}
	}

	return Ipv4Interface{}, fmt.Errorf("could not get interface by index: %v", idx)
}

// GetInterfaces gets a list of interfaces and addresses
func GetInterfaces() (ifaces []Ipv4Interface, err error) {
	defer func() {
		if r := recover(); r != nil {
			if err == nil {
				err = fmt.Errorf("panic while getting all interfaces: %v", r)
			}
		}
	}()

	// find out how big our buffer needs to be
	b := make([]byte, 1)
	ai := (*syscall.IpAdapterInfo)(unsafe.Pointer(&b[0]))
	ol := uint32(0)
	syscall.GetAdaptersInfo(ai, &ol)

	// start to get info
	b = make([]byte, 1)
	ai = (*syscall.IpAdapterInfo)(unsafe.Pointer(&b[0]))
	if err := syscall.GetAdaptersInfo(ai, &ol); err != nil {
		return nil, fmt.Errorf("failed to get all interfaces: could not call system GetAdaptersInfo: %v", err)
	}

	// iterate to find
	var ifacePointers []*Ipv4Interface
	for ; ai != nil; ai = ai.Next {
		if ai.Type != windows.IF_TYPE_ETHERNET_CSMACD {
			continue
		}

		aiIndex := int(ai.Index)
		aiDescription := converters.UnsafeUTF16BytesToString(ai.Description[:])
		aiDhcpEnabled := int(ai.DhcpEnabled)

		var aiAddress, aiMask string
		for ipl := &ai.IpAddressList; ipl != nil; ipl = ipl.Next {
			aiAddress = converters.UnsafeUTF16BytesToString(ipl.IpAddress.String[:])
			aiMask = converters.UnsafeUTF16BytesToString(ipl.IpMask.String[:])
			if aiAddress != "" && aiMask != "" {
				break
			}
		}

		var aiGatewayAddress string
		for gwl := &ai.GatewayList; gwl != nil; gwl = gwl.Next {
			aiGatewayAddress = converters.UnsafeUTF16BytesToString(gwl.IpAddress.String[:])
		}

		ifacePointers = append(ifacePointers, &Ipv4Interface{
			Idx:                   aiIndex,
			Description:           aiDescription,
			IpAddress:             aiAddress,
			IpMask:                aiMask,
			DefaultGatewayAddress: aiGatewayAddress,
			DhcpEnabled:           aiDhcpEnabled == 1,
		})
	}

	// deep copy
	for _, ifacePointer := range ifacePointers {
		// gain nets nets friendly name
		iface, err := net.InterfaceByIndex(ifacePointer.Idx)
		if err != nil {
			continue
		}

		ifaces = append(ifaces, Ipv4Interface{
			Idx:                   ifacePointer.Idx,
			Name:                  iface.Name,
			Description:           fmt.Sprint(ifacePointer.Description),
			IpAddress:             fmt.Sprint(ifacePointer.IpAddress),
			IpMask:                fmt.Sprint(ifacePointer.IpMask),
			DefaultGatewayAddress: fmt.Sprint(ifacePointer.DefaultGatewayAddress),
			DhcpEnabled:           ifacePointer.DhcpEnabled,
		})
	}

	return ifaces, nil
}

// EnableForwardingByIndex enables the interface forwarding by index
func EnableForwardingByIndex(idx int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if err == nil {
				err = fmt.Errorf("panic while enabling forwarding on interface %d: %v", idx, r)
			}
		}
	}()

	ifRow := &syscalls.MibIpInterfaceRow{
		Family:         syscall.AF_INET, // IPv4
		InterfaceIndex: uint32(idx),
	}
	if err := syscalls.GetIpInterfaceEntry(ifRow); err != nil {
		return fmt.Errorf("failed to enable forwarding on interface %d: could not get interface: %v", idx, err)
	}
	if ifRow.ForwardingEnabled == uint8(1) {
		return nil
	}

	ifRow.SitePrefixLength = uint32(0) // must be set to 0
	ifRow.ForwardingEnabled = uint8(1) // forwarding enable
	if err := syscalls.SetIpInterfaceEntry(ifRow); err != nil {
		return fmt.Errorf("failed to enable forwarding on interface %d: could not set interface: %v", idx, err)
	}

	return nil
}

// EnableForwardingByName enables the interface forwarding by name
func EnableForwardingByName(name string) error {
	iface, err := GetInterfaceByName(name)
	if err != nil {
		return err
	}

	return EnableForwardingByIndex(iface.Idx)
}

// GetDefaultGatewayIfaceIndex get the interface index that has the default gateway
func GetDefaultGatewayIfaceIndex() (ifaceIdx int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if err == nil {
				err = fmt.Errorf("panic while getting default nets index: %v", r)
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
		return -1, fmt.Errorf("could not call system GetIpForwardTable: %v", err)
	}

	// iterate to find
	for i := 0; i < int(ft.NumEntries); i++ {
		row := *(*syscalls.MibIpForwardRow)(unsafe.Pointer(
			uintptr(unsafe.Pointer(&ft.Table[0])) + uintptr(i)*uintptr(unsafe.Sizeof(ft.Table[0])), // head idx + offset
		))

		if converters.Inet_ntoa(row.ForwardDest, false) != "0.0.0.0" {
			continue
		}

		return int(row.ForwardIfIndex), nil
	}

	return -1, errors.New("there isn't a default gateway with a destination of 0.0.0.0")
}
