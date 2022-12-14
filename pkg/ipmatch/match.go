// Copyright 2022 flannel authors
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

package ipmatch

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	log "k8s.io/klog"
)

const (
	ipv4Stack int = iota
	ipv6Stack
	dualStack
	noneStack
)

type PublicIPOpts struct {
	PublicIP   string
	PublicIPv6 string
}

func GetIPFamily(autoDetectIPv4, autoDetectIPv6 bool) (int, error) {
	if autoDetectIPv4 && !autoDetectIPv6 {
		return ipv4Stack, nil
	} else if !autoDetectIPv4 && autoDetectIPv6 {
		return ipv6Stack, nil
	} else if autoDetectIPv4 && autoDetectIPv6 {
		return dualStack, nil
	}
	return noneStack, errors.New("none defined stack")
}

func LookupExtIface(ifname string, ifregexS string, ifcanreach string, ipStack int, opts PublicIPOpts) (*backend.ExternalInterface, error) {
	var iface *net.Interface
	var ifaceAddr net.IP
	var ifaceV6Addr net.IP
	var err error
	var ifregex *regexp.Regexp

	if ifregexS != "" {
		ifregex, err = regexp.Compile(ifregexS)
		if err != nil {
			return nil, fmt.Errorf("could not compile the IP address regex '%s': %w", ifregexS, err)
		}
	}

	// Check ip family stack
	if ipStack == noneStack {
		return nil, fmt.Errorf("none matched ip stack")
	}

	if len(ifname) > 0 {
		if ifaceAddr = net.ParseIP(ifname); ifaceAddr != nil {
			log.Infof("Searching for interface using %s", ifaceAddr)
			switch ipStack {
			case ipv4Stack:
				iface, err = ip.GetInterfaceByIP(ifaceAddr)
				if err != nil {
					return nil, fmt.Errorf("error looking up interface %s: %s", ifname, err)
				}
			case ipv6Stack:
				iface, err = ip.GetInterfaceByIP6(ifaceAddr)
				if err != nil {
					return nil, fmt.Errorf("error looking up v6 interface %s: %s", ifname, err)
				}
			case dualStack:
				if ifaceAddr.To4() != nil {
					iface, err = ip.GetInterfaceByIP(ifaceAddr)
					if err != nil {
						return nil, fmt.Errorf("error looking up interface %s: %s", ifname, err)
					}
				}
				if len(opts.PublicIPv6) > 0 {
					if ifaceV6Addr = net.ParseIP(opts.PublicIPv6); ifaceV6Addr != nil {
						v6Iface, err := ip.GetInterfaceByIP6(ifaceV6Addr)
						if err != nil {
							return nil, fmt.Errorf("error looking up v6 interface %s: %s", opts.PublicIPv6, err)
						}
						if ifaceAddr.To4() == nil {
							iface = v6Iface
							ifaceAddr = nil
						} else {
							if iface.Name != v6Iface.Name {
								return nil, fmt.Errorf("v6 interface %s must be the same with v4 interface %s", v6Iface.Name, iface.Name)
							}
						}
					}
				}
			}
		} else {
			iface, err = net.InterfaceByName(ifname)
			if err != nil {
				return nil, fmt.Errorf("error looking up interface %s: %s", ifname, err)
			}
		}
	} else if ifregex != nil {
		// Use the regex if specified and the iface option for matching a specific ip or name is not used
		ifaces, err := net.Interfaces()
		if err != nil {
			return nil, fmt.Errorf("error listing all interfaces: %s", err)
		}

		// Check IP
	ifaceLoop:
		for _, ifaceToMatch := range ifaces {
			switch ipStack {
			case ipv4Stack:
				ifaceIPs, err := ip.GetInterfaceIP4Addrs(&ifaceToMatch)
				if err != nil {
					// Skip if there is no IPv4 address
					continue
				}
				if matched := matchIP(ifregex, ifaceIPs); matched != nil {
					ifaceAddr = matched
					iface = &ifaceToMatch
					break ifaceLoop
				}
			case ipv6Stack:
				ifaceIPs, err := ip.GetInterfaceIP6Addrs(&ifaceToMatch)
				if err != nil {
					// Skip if there is no IPv6 address
					continue
				}
				if matched := matchIP(ifregex, ifaceIPs); matched != nil {
					ifaceV6Addr = matched
					iface = &ifaceToMatch
					break ifaceLoop
				}
			case dualStack:
				ifaceIPs, err := ip.GetInterfaceIP4Addrs(&ifaceToMatch)
				if err != nil {
					// Skip if there is no IPv4 address
					continue
				}

				ifaceV6IPs, err := ip.GetInterfaceIP6Addrs(&ifaceToMatch)
				if err != nil {
					// Skip if there is no IPv6 address
					continue
				}

				if matched := matchIP(ifregex, ifaceIPs); matched != nil {
					ifaceAddr = matched
				} else {
					continue
				}
				if matched := matchIP(ifregex, ifaceV6IPs); matched != nil {
					ifaceV6Addr = matched
					iface = &ifaceToMatch
					break ifaceLoop
				}
			}
		}

		// Check Name
		if iface == nil && (ifaceAddr == nil || ifaceV6Addr == nil) {
			for _, ifaceToMatch := range ifaces {
				if ifregex.MatchString(ifaceToMatch.Name) {
					iface = &ifaceToMatch
					break
				}
			}
		}

		// Check that nothing was matched
		if iface == nil {
			var availableFaces []string
			for _, f := range ifaces {
				var ipaddr []net.IP
				switch ipStack {
				case ipv4Stack, dualStack:
					ipaddr, _ = ip.GetInterfaceIP4Addrs(&f) // We can safely ignore errors. We just won't log any ip
				case ipv6Stack:
					ipaddr, _ = ip.GetInterfaceIP6Addrs(&f) // We can safely ignore errors. We just won't log any ip
				}
				availableFaces = append(availableFaces, fmt.Sprintf("%s:%v", f.Name, ipaddr))
			}

			return nil, fmt.Errorf("Could not match pattern %s to any of the available network interfaces (%s)", ifregexS, strings.Join(availableFaces, ", "))
		}
	} else if len(ifcanreach) > 0 {
		log.Info("Determining interface to use based on given ifcanreach: ", ifcanreach)
		if iface, ifaceAddr, err = ip.GetInterfaceBySpecificIPRouting(net.ParseIP(ifcanreach)); err != nil {
			return nil, fmt.Errorf("failed to get ifcanreach based interface: %s", err)
		}
	} else {
		log.Info("Determining IP address of default interface")
		switch ipStack {
		case ipv4Stack:
			if iface, err = ip.GetDefaultGatewayInterface(); err != nil {
				return nil, fmt.Errorf("failed to get default interface: %w", err)
			}
		case ipv6Stack:
			if iface, err = ip.GetDefaultV6GatewayInterface(); err != nil {
				return nil, fmt.Errorf("failed to get default v6 interface: %w", err)
			}
		case dualStack:
			if iface, err = ip.GetDefaultGatewayInterface(); err != nil {
				return nil, fmt.Errorf("failed to get default interface: %w", err)
			}
			v6Iface, err := ip.GetDefaultV6GatewayInterface()
			if err != nil {
				return nil, fmt.Errorf("failed to get default v6 interface: %w", err)
			}
			if iface.Name != v6Iface.Name {
				return nil, fmt.Errorf("v6 default route interface %s "+
					"must be the same with v4 default route interface %s", v6Iface.Name, iface.Name)
			}
		}
	}

	var ifaceAddrs []net.IP
	var ifaceV6Addrs []net.IP
	if ipStack == ipv4Stack && ifaceAddr == nil {
		ifaceAddrs, err = ip.GetInterfaceIP4Addrs(iface)
		if err != nil || len(ifaceAddrs) == 0 {
			return nil, fmt.Errorf("failed to find IPv4 address for interface %s", iface.Name)
		}
		ifaceAddr = ifaceAddrs[0]
	} else if ipStack == ipv6Stack && ifaceV6Addr == nil {
		ifaceV6Addrs, err = ip.GetInterfaceIP6Addrs(iface)
		if err != nil || len(ifaceV6Addrs) == 0 {
			return nil, fmt.Errorf("failed to find IPv6 address for interface %s", iface.Name)
		}
		ifaceV6Addr = ifaceV6Addrs[0]
	} else if ipStack == dualStack && ifaceAddr == nil && ifaceV6Addr == nil {
		ifaceAddrs, err = ip.GetInterfaceIP4Addrs(iface)
		if err != nil || len(ifaceAddrs) == 0 {
			return nil, fmt.Errorf("failed to find IPv4 address for interface %s", iface.Name)
		}
		ifaceAddr = ifaceAddrs[0]
		ifaceV6Addrs, err = ip.GetInterfaceIP6Addrs(iface)
		if err != nil || len(ifaceV6Addrs) == 0 {
			return nil, fmt.Errorf("failed to find IPv6 address for interface %s", iface.Name)
		}
		ifaceV6Addr = ifaceV6Addrs[0]
	}

	if ifaceAddr != nil {
		log.Infof("Using interface with name %s and address %s", iface.Name, ifaceAddr)
	}
	if ifaceV6Addr != nil {
		log.Infof("Using interface with name %s and v6 address %s", iface.Name, ifaceV6Addr)
	}

	if iface.MTU == 0 {
		return nil, fmt.Errorf("failed to determine MTU for %s interface", ifaceAddr)
	}

	var extAddr net.IP
	var extV6Addr net.IP

	if len(opts.PublicIP) > 0 {
		extAddr = net.ParseIP(opts.PublicIP)
		if extAddr == nil {
			return nil, fmt.Errorf("invalid public IP address: %s", opts.PublicIP)
		}
		log.Infof("Using %s as external address", extAddr)
	}

	if extAddr == nil && ipStack != ipv6Stack {
		log.Infof("Defaulting external address to interface address (%s)", ifaceAddr)
		extAddr = ifaceAddr
	}

	if len(opts.PublicIPv6) > 0 {
		extV6Addr = net.ParseIP(opts.PublicIPv6)
		if extV6Addr == nil {
			return nil, fmt.Errorf("invalid public IPv6 address: %s", opts.PublicIPv6)
		}
		log.Infof("Using %s as external address", extV6Addr)
	}

	if extV6Addr == nil && ipStack != ipv4Stack {
		log.Infof("Defaulting external v6 address to interface address (%s)", ifaceV6Addr)
		extV6Addr = ifaceV6Addr
	}

	return &backend.ExternalInterface{
		Iface:       iface,
		IfaceAddr:   ifaceAddr,
		IfaceV6Addr: ifaceV6Addr,
		ExtAddr:     extAddr,
		ExtV6Addr:   extV6Addr,
	}, nil
}

func matchIP(ifregex *regexp.Regexp, ifaceIPs []net.IP) net.IP {
	for _, ifaceIP := range ifaceIPs {
		if ifregex.MatchString(ifaceIP.String()) {
			return ifaceIP
		}
	}
	return nil
}
