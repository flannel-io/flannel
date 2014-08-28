package subnet

import (
	"encoding/json"
	"errors"

	"github.com/coreos/rudder/pkg/ip"
)

type Config struct {
	Network   ip.IP4Net
	SubnetMin ip.IP4
	SubnetMax ip.IP4
	SubnetLen uint
}

func ParseConfig(s string) (*Config, error) {
	cfg := new(Config)
	err := json.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}

	if cfg.SubnetLen > 0 {
		if cfg.SubnetLen < cfg.Network.PrefixLen {
			return nil, errors.New("HostSubnet is larger network than Network")
		}
	} else {
		// try to give each host a /24 but if the whole network
		// is /24 or smaller, half the network
		if cfg.Network.PrefixLen < 24 {
			cfg.SubnetLen = 24
		} else {
			cfg.SubnetLen = cfg.Network.PrefixLen + 1
		}
	}

	subnetSize := ip.IP4(1 << (32 - cfg.SubnetLen))

	if cfg.SubnetMin == ip.IP4(0) {
		// skip over the first subnet otherwise it causes problems. e.g.
		// if Network is 10.100.0.0/16, having an interface with 10.0.0.0
		// makes ping think it's a broadcast address (not sure why)
		cfg.SubnetMin = cfg.Network.IP + subnetSize
	} else if !cfg.Network.Contains(cfg.SubnetMin) {
		return nil, errors.New("SubnetMin is not in the range of the Network")
	}

	if cfg.SubnetMax == ip.IP4(0) {
		cfg.SubnetMax = cfg.Network.Next().IP - subnetSize
	} else if !cfg.Network.Contains(cfg.SubnetMax) {
		return nil, errors.New("SubnetMax is not in the range of the Network")
	}

	return cfg, nil
}
