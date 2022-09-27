//go:build !windows
// +build !windows

// Copyright 2021 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package wireguard

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/flannel-io/flannel/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/subnet"
	"golang.org/x/net/context"
)

type Mode string

const (
	Separate Mode = "separate"
	Auto     Mode = "auto"
	Ipv4     Mode = "ipv4"
	Ipv6     Mode = "ipv6"
)

func init() {
	backend.Register("wireguard", New)
}

type WireguardBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := &WireguardBackend{
		sm:       sm,
		extIface: extIface,
	}

	return be, nil
}

func newSubnetAttrs(publicIP net.IP, publicIPv6 net.IP, enableIPv4, enableIPv6 bool, publicKey string) (*subnet.LeaseAttrs, error) {
	data, err := json.Marshal(&wireguardLeaseAttrs{
		PublicKey: publicKey,
	})
	if err != nil {
		return nil, err
	}

	leaseAttrs := &subnet.LeaseAttrs{
		BackendType: "wireguard",
	}

	if publicIP != nil {
		leaseAttrs.PublicIP = ip.FromIP(publicIP)
	}

	if enableIPv4 {
		leaseAttrs.BackendData = json.RawMessage(data)
	}

	if publicIPv6 != nil {
		leaseAttrs.PublicIPv6 = ip.FromIP6(publicIPv6)
	}

	if enableIPv6 {
		leaseAttrs.BackendV6Data = json.RawMessage(data)
	}

	return leaseAttrs, nil
}

func createWGDev(ctx context.Context, wg *sync.WaitGroup, name string, psk string, keepalive *time.Duration, listenPort int, mtu int) (*wgDevice, error) {
	devAttrs := wgDeviceAttrs{
		keepalive:  keepalive,
		listenPort: listenPort,
		name:       name,
		MTU:        mtu,
	}
	err := devAttrs.setupKeys(psk)
	if err != nil {
		return nil, err
	}
	return newWGDevice(&devAttrs, ctx, wg)
}

func (be *WireguardBackend) RegisterNetwork(ctx context.Context, wg *sync.WaitGroup, config *subnet.Config) (backend.Network, error) {
	// Parse out configuration
	cfg := struct {
		ListenPort                  int
		ListenPortV6                int
		PSK                         string
		PersistentKeepaliveInterval time.Duration
		Mode                        Mode
	}{
		ListenPort:                  51820,
		ListenPortV6:                51821,
		PersistentKeepaliveInterval: 0,
		Mode:                        Separate,
	}

	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding backend config: %w", err)
		}
	}

	keepalive := cfg.PersistentKeepaliveInterval * time.Second

	var err error
	var dev, v6Dev *wgDevice
	var publicKey string
	if cfg.Mode == Separate {
		if config.EnableIPv4 {
			dev, err = createWGDev(ctx, wg, "flannel-wg", cfg.PSK, &keepalive, cfg.ListenPort, be.extIface.Iface.MTU)
			if err != nil {
				return nil, err
			}
			publicKey = dev.attrs.publicKey.String()
		}
		if config.EnableIPv6 {
			v6Dev, err = createWGDev(ctx, wg, "flannel-wg-v6", cfg.PSK, &keepalive, cfg.ListenPortV6, be.extIface.Iface.MTU)
			if err != nil {
				return nil, err
			}
			publicKey = v6Dev.attrs.publicKey.String()
		}
	} else if cfg.Mode == Auto || cfg.Mode == Ipv4 || cfg.Mode == Ipv6 {
		dev, err = createWGDev(ctx, wg, "flannel-wg", cfg.PSK, &keepalive, cfg.ListenPort, be.extIface.Iface.MTU)
		if err != nil {
			return nil, err
		}
		publicKey = dev.attrs.publicKey.String()
	} else {
		return nil, fmt.Errorf("No valid Mode configured")
	}

	subnetAttrs, err := newSubnetAttrs(be.extIface.ExtAddr, be.extIface.ExtV6Addr, config.EnableIPv4, config.EnableIPv6, publicKey)
	if err != nil {
		return nil, err
	}

	lease, err := be.sm.AcquireLease(ctx, subnetAttrs)
	switch err {
	case nil:
	case context.Canceled, context.DeadlineExceeded:
		return nil, err
	default:
		return nil, fmt.Errorf("failed to acquire lease: %w", err)

	}

	if config.EnableIPv4 {
		err = dev.Configure(lease.Subnet.IP, subnet.GetFlannelNetwork(config))
		if err != nil {
			return nil, err
		}
	}

	if config.EnableIPv6 {
		if cfg.Mode == Separate {
			err = v6Dev.ConfigureV6(lease.IPv6Subnet.IP, subnet.GetFlannelIPv6Network(config))
		} else {
			err = dev.ConfigureV6(lease.IPv6Subnet.IP, subnet.GetFlannelIPv6Network(config))
		}
		if err != nil {
			return nil, err
		}
	}

	return newNetwork(be.sm, be.extIface, dev, v6Dev, cfg.Mode, lease)
}
