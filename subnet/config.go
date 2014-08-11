package subnet

import (
	"encoding/json"
	"errors"

	"github.com/coreos-inc/kolach/pkg"
)

type Config struct {
	Network    pkg.IP4Net
	FirstIP    pkg.IP4
	LastIP     pkg.IP4
	HostSubnet uint
}

func ParseConfig(s string) (*Config, error) {
	cfg := new(Config)
	err := json.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}

	if cfg.HostSubnet > 0 {
		if cfg.HostSubnet < cfg.Network.PrefixLen {
			return nil, errors.New("HostSubnet is larger network than Network")
		}
	} else {
		// try to give each host a /24 but if the whole network
		// is /24 or smaller, half the network
		if cfg.Network.PrefixLen < 24 {
			cfg.HostSubnet = 24
		} else {
			cfg.HostSubnet = cfg.Network.PrefixLen + 1
		}
	}

	if cfg.FirstIP == pkg.IP4(0) {
		cfg.FirstIP = cfg.Network.IP
	} else if !cfg.Network.Contains(cfg.FirstIP) {
		return nil, errors.New("FirstIP is not in the range of the Network")
	}

	if cfg.LastIP == pkg.IP4(0) {
		cfg.LastIP = cfg.Network.Next().IP
		cfg.LastIP -= (1 << (32 - cfg.HostSubnet))
	} else if !cfg.Network.Contains(cfg.LastIP) {
		return nil, errors.New("LastIP is not in the range of the Network")
	}

	return cfg, nil
}
