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

package subnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"

	"github.com/flannel-io/flannel/pkg/ip"
	"k8s.io/klog"
	netutils "k8s.io/utils/net"
)

type Config struct {
	EnableIPv4    bool
	EnableIPv6    bool
	Network       ip.IP4Net
	IPv6Network   ip.IP6Net
	Networks      []ip.IP4Net
	IPv6Networks  []ip.IP6Net
	SubnetMin     ip.IP4
	SubnetMax     ip.IP4
	IPv6SubnetMin *ip.IP6
	IPv6SubnetMax *ip.IP6
	SubnetLen     uint
	IPv6SubnetLen uint
	BackendType   string          `json:"-"`
	Backend       json.RawMessage `json:",omitempty"`
}

func parseBackendType(be json.RawMessage) (string, error) {
	var bt struct {
		Type string
	}

	if len(be) == 0 {
		return "udp", nil
	} else if err := json.Unmarshal(be, &bt); err != nil {
		return "", fmt.Errorf("error decoding Backend property of config: %v", err)
	}

	return bt.Type, nil
}

func ParseConfig(s string) (*Config, error) {
	cfg := new(Config)
	// Enable ipv4 by default
	cfg.EnableIPv4 = true
	err := json.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}

	bt, err := parseBackendType(cfg.Backend)
	if err != nil {
		return nil, err
	}
	cfg.BackendType = bt

	cfg.Networks = make([]ip.IP4Net, 0)
	cfg.IPv6Networks = make([]ip.IP6Net, 0)

	return cfg, nil
}

// CheckNetworkConfig checks the coherence of the flannel configuration.
// It is used only with the local network manager, not with the kubernetes-based manager.
func CheckNetworkConfig(config *Config) error {
	if config.EnableIPv4 {
		if config.Network.Empty() {
			return errors.New("please define a correct Network parameter in the flannel config")
		}
		if config.SubnetLen > 0 {
			// SubnetLen needs to allow for a tunnel and bridge device on each host.
			if config.SubnetLen > 30 {
				return errors.New("SubnetLen must be less than /31")
			}

			// SubnetLen needs to fit _more_ than twice into the Network.
			// the first subnet isn't used, so splitting into two one only provide one usable host.
			if config.SubnetLen < config.Network.PrefixLen+2 {
				return errors.New("network must be able to accommodate at least four subnets")
			}
		} else {
			// If the network is smaller than a /28 then the network isn't big enough for flannel so return an error.
			// Default to giving each host at least a /24 (as long as the network is big enough to support at least four hosts)
			// Otherwise, if the network is too small to give each host a /24 just split the network into four.
			if config.Network.PrefixLen > 28 {
				// Each subnet needs at least four addresses (/30) and the network needs to accommodate at least four
				// since the first subnet isn't used, so splitting into two would only provide one usable host.
				// So the min useful PrefixLen is /28
				return errors.New("network is too small. Minimum useful network prefix is /28")
			} else if config.Network.PrefixLen <= 22 {
				// Network is big enough to give each host a /24
				config.SubnetLen = 24
			} else {
				// Use +2 to provide four hosts per subnet.
				config.SubnetLen = config.Network.PrefixLen + 2
			}
		}

		subnetSize := ip.IP4(1 << (32 - config.SubnetLen))

		if config.SubnetMin == ip.IP4(0) {
			// skip over the first subnet otherwise it causes problems. e.g.
			// if Network is 10.100.0.0/16, having an interface with 10.100.0.0
			// conflicts with the network address.
			config.SubnetMin = config.Network.IP + subnetSize
		} else if !config.Network.Contains(config.SubnetMin) {
			return errors.New("SubnetMin is not in the range of the Network")
		}

		if config.SubnetMax == ip.IP4(0) {
			config.SubnetMax = config.Network.Next().IP - subnetSize
		} else if !config.Network.Contains(config.SubnetMax) {
			return errors.New("SubnetMax is not in the range of the Network")
		}

		// The SubnetMin and SubnetMax need to be aligned to a SubnetLen boundary
		mask := ip.IP4(0xFFFFFFFF << (32 - config.SubnetLen))
		if config.SubnetMin != config.SubnetMin&mask {
			return fmt.Errorf("SubnetMin is not on a SubnetLen boundary: %v", config.SubnetMin)
		}

		if config.SubnetMax != config.SubnetMax&mask {
			return fmt.Errorf("SubnetMax is not on a SubnetLen boundary: %v", config.SubnetMax)
		}
	}
	if config.EnableIPv6 {
		if config.IPv6Network.Empty() {
			return errors.New("please define a correct IPv6Network parameter in the flannel config")
		}
		if config.IPv6SubnetLen > 0 {
			// SubnetLen needs to allow for a tunnel and bridge device on each host.
			if config.IPv6SubnetLen > 126 {
				return errors.New("SubnetLen must be less than /127")
			}

			// SubnetLen needs to fit _more_ than twice into the Network.
			// the first subnet isn't used, so splitting into two one only provide one usable host.
			if config.IPv6SubnetLen < config.IPv6Network.PrefixLen+2 {
				return errors.New("network must be able to accommodate at least four subnets")
			}
		} else {
			// If the network is smaller than a /124 then the network isn't big enough for flannel so return an error.
			// Default to giving each host at least a /64 (as long as the network is big enough to support at least four hosts)
			// Otherwise, if the network is too small to give each host a /64 just split the network into four.
			if config.IPv6Network.PrefixLen > 124 {
				// Each subnet needs at least four addresses (/126) and the network needs to accommodate at least four
				// since the first subnet isn't used, so splitting into two would only provide one usable host.
				// So the min useful PrefixLen is /124
				return errors.New("IPv6Network is too small. Minimum useful network prefix is /124")
			} else if config.IPv6Network.PrefixLen <= 62 {
				// Network is big enough to give each host a /64
				config.IPv6SubnetLen = 64
			} else {
				// Use +2 to provide four hosts per subnet.
				config.IPv6SubnetLen = config.IPv6Network.PrefixLen + 2
			}
		}

		ipv6SubnetSize := big.NewInt(0).Lsh(big.NewInt(1), 128-config.IPv6SubnetLen)

		if ip.IsEmpty(config.IPv6SubnetMin) {
			// skip over the first subnet otherwise it causes problems. e.g.
			// if Network is fc00::/48, having an interface with fc00::
			// conflicts with the broadcast address.
			config.IPv6SubnetMin = ip.GetIPv6SubnetMin(config.IPv6Network.IP, ipv6SubnetSize)
		} else if !config.IPv6Network.Contains(config.IPv6SubnetMin) {
			return errors.New("IPv6SubnetMin is not in the range of the IPv6Network")
		}

		if ip.IsEmpty(config.IPv6SubnetMax) {
			config.IPv6SubnetMax = ip.GetIPv6SubnetMax(config.IPv6Network.Next().IP, ipv6SubnetSize)
		} else if !config.IPv6Network.Contains(config.IPv6SubnetMax) {
			return errors.New("IPv6SubnetMax is not in the range of the IPv6Network")
		}

		// The SubnetMin and SubnetMax need to be aligned to a SubnetLen boundary
		mask := ip.Mask(int(config.IPv6SubnetLen))
		if !ip.CheckIPv6Subnet(config.IPv6SubnetMin, mask) {
			return fmt.Errorf("IPv6SubnetMin is not on a SubnetLen boundary: %v", config.IPv6SubnetMin)
		}

		if !ip.CheckIPv6Subnet(config.IPv6SubnetMax, mask) {
			return fmt.Errorf("IPv6SubnetMax is not on a SubnetLen boundary: %v", config.IPv6SubnetMax)
		}
	}
	return nil
}

// GetFlannelNetwork returns the relevant IPv4 network (i.e. clusterCIDR) for subnet sn
// If Networks is not empty, GetFlannelNetwork returns the first networks that contains subnet sn.
// If Networks is empty, this means we are not using the MultiClusterCIDR API
// so GetFlannelNetwork falls back to the standard behavior and returns the single Network entry
func (c *Config) GetFlannelNetwork(sn *ip.IP4Net) (ip.IP4Net, error) {
	if c.HasNetworks() {
		for _, net := range c.Networks {
			if net.ContainsCIDR(sn) {
				return net, nil
			}
		}
		return ip.IP4Net{}, fmt.Errorf("could not find flannel networks matching subnet %s", sn)
	} else {
		emptyNet := ip.IP4Net{}
		if c.Network != emptyNet {
			return c.Network, nil
		} else {
			return emptyNet, fmt.Errorf("could not find an ipv4 network in the flannel configuration")
		}
	}
}

// GetFlannelIPv6Network returns the relevant IPv6 network (i.e. clusterCIDR) for subnet sn
// If Networks is not empty, GetFlannelIPv6Network returns the first networks that contains subnet sn.
// If Networks is empty, this means we are not using the MultiClusterCIDR API
// so GetFlannelIPv6Network falls back to the standard behavior and returns the single IPv6Network entry
func (c *Config) GetFlannelIPv6Network(sn *ip.IP6Net) (ip.IP6Net, error) {
	if c.HasIPv6Networks() {
		for _, net := range c.IPv6Networks {
			if net.ContainsCIDR(sn) {
				return net, nil
			}
		}
		return ip.IP6Net{}, fmt.Errorf("could not find flannel ipv6 networks matching subnet %s", sn)
	} else {
		emptyNet := ip.IP6Net{}
		if c.IPv6Network != emptyNet {
			return c.IPv6Network, nil
		} else {
			return emptyNet, fmt.Errorf("could not find an ipv6 network in the flannel configuration")
		}

	}
}

// AddNetwork adds net to either c.Networks or c.IPv6Networks depending on its type
func (c *Config) AddNetwork(net *net.IPNet) {
	if netutils.IsIPv4CIDR(net) {
		ip4net := ip.FromIPNet(net)
		if !c.containsIPv4Network(ip4net) {
			c.Networks = append(c.Networks, ip4net)
		}
	} else if netutils.IsIPv6CIDR(net) {
		ip6net := ip.FromIP6Net(net)
		if !c.containsIPv6Network(ip6net) {
			c.IPv6Networks = append(c.IPv6Networks, ip6net)
		}
	} else {
		klog.Warningf("cannot add unknown CIDR to config: %s", net)
	}
}

func (c *Config) containsIPv4Network(net ip.IP4Net) bool {
	for _, ip4net := range c.Networks {
		if net.Equal(ip4net) {
			return true
		}
	}
	return false
}

func (c *Config) containsIPv6Network(net ip.IP6Net) bool {
	for _, ip6net := range c.IPv6Networks {
		if net.Equal(ip6net) {
			return true
		}
	}
	return false
}

// HasNetworks returns true if there is at least 1 IPv4 network in the flannel config,
// false otherwise
func (c *Config) HasNetworks() bool {
	if c.Networks != nil {
		return len(c.Networks) > 0
	} else {
		return false
	}
}

// HasIPv6Networks returns true if there is at least 1 IPv6 network in the flannel config,
// false otherwise
func (c *Config) HasIPv6Networks() bool {
	if c.IPv6Networks != nil {
		return len(c.IPv6Networks) > 0
	} else {
		return false
	}
}
