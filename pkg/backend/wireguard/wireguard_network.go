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
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/flannel-io/flannel/pkg/subnet"
	log "k8s.io/klog/v2"
)

const (
	/*
		20-byte IPv4 header or 40 byte IPv6 header
		8-byte UDP header
		4-byte type
		4-byte key index
		8-byte nonce
		N-byte encrypted data
		16-byte authentication tag
	*/
	overhead = 80
)

type network struct {
	dev      *wgDevice
	v6Dev    *wgDevice
	extIface *backend.ExternalInterface
	mode     Mode
	lease    *lease.Lease
	sm       subnet.Manager
	mtu      int
}

func newNetwork(sm subnet.Manager, extIface *backend.ExternalInterface, dev, v6Dev *wgDevice, mode Mode, lease *lease.Lease, mtu int) (*network, error) {
	n := &network{
		dev:      dev,
		v6Dev:    v6Dev,
		extIface: extIface,
		mode:     mode,
		lease:    lease,
		sm:       sm,
		mtu:      mtu,
	}

	return n, nil
}

func (n *network) Lease() *lease.Lease {
	return n.lease
}

func (n *network) MTU() int {
	return n.mtu - overhead
}

func (n *network) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.Info("Watching for new subnet leases")
	events := make(chan []lease.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, n.sm, n.lease, events)
		wg.Done()
	}()

	defer wg.Wait()

	for {
		select {
		case evtBatch := <-events:
			n.handleSubnetEvents(ctx, evtBatch)

		case <-ctx.Done():
			return
		}
	}
}

type wireguardLeaseAttrs struct {
	PublicKey string
	Port      uint16
}

// Select the mode that is most likely to allow for a successful connection.
// If both ipv4 and ipv6 addresses are provided:
//   - Prefer ipv4 if the remote endpoint has a public ipv4 address
//     and the external iface has an ipv4 address as well. Anything with
//     an ipv4 address can likely connect to the public internet.
//   - Use ipv6 if the remote endpoint has a publc address and the local
//     interface has a public address as well. In which case it's likely that
//     a connection can be made. The local interface having just an link-local
//     address will only have a small chance of succeeding (ipv6 masquarading is
//     very rare)
//   - If neither is true default to ipv4 and cross fingers.
func (n *network) selectMode(ip4 ip.IP4, ip6 *ip.IP6) Mode {
	if ip6 == nil {
		return Ipv4
	}
	if !ip4.IsPrivate() && n.extIface.ExtAddr != nil {
		return Ipv4
	}
	if !ip6.IsPrivate() && n.extIface.ExtV6Addr != nil && !ip.FromIP6(n.extIface.ExtV6Addr).IsPrivate() {
		return Ipv6
	}
	return Ipv4
}

func (n *network) handleSubnetEvents(ctx context.Context, batch []lease.Event) {
	for _, event := range batch {
		switch event.Type {
		case lease.EventAdded:

			if event.Lease.Attrs.BackendType != "wireguard" {
				log.Warningf("Ignoring non-wireguard subnet: type=%v", event.Lease.Attrs.BackendType)
				continue
			}

			var v4wireguardAttrs, v6wireguardAttrs, wireguardAttrs wireguardLeaseAttrs
			var subnets []*net.IPNet
			if event.Lease.EnableIPv4 {
				if len(event.Lease.Attrs.BackendData) > 0 {
					if err := json.Unmarshal(event.Lease.Attrs.BackendData, &v4wireguardAttrs); err != nil {
						log.Errorf("failed to unmarshal BackendData: %v", err)
						continue
					}
				}
				wireguardAttrs = v4wireguardAttrs
				subnets = append(subnets, event.Lease.Subnet.ToIPNet()) // only used if n.mode != Separate
			}

			if event.Lease.EnableIPv6 {
				if len(event.Lease.Attrs.BackendV6Data) > 0 {
					if err := json.Unmarshal(event.Lease.Attrs.BackendV6Data, &v6wireguardAttrs); err != nil {
						log.Errorf("failed to unmarshal BackendData: %v", err)
						continue
					}
				}
				wireguardAttrs = v6wireguardAttrs
				subnets = append(subnets, event.Lease.IPv6Subnet.ToIPNet()) // only used if n.mode != Separate
			}

			// default to the port in the attr, but use the device's listen port
			// if it's not set for backwards compatibility with older flannel
			// versions.
			v4Port := v4wireguardAttrs.Port
			if v4Port == 0 && n.dev != nil {
				v4Port = uint16(n.dev.attrs.listenPort)
			}
			v6Port := v6wireguardAttrs.Port
			if v6Port == 0 && n.v6Dev != nil {
				v6Port = uint16(n.v6Dev.attrs.listenPort)
			}
			v4PeerEndpoint := fmt.Sprintf("%s:%d", event.Lease.Attrs.PublicIP.String(), v4Port)
			var v6PeerEndpoint string
			if event.Lease.Attrs.PublicIPv6 != nil {
				v6PeerEndpoint = fmt.Sprintf("[%s]:%d", event.Lease.Attrs.PublicIPv6.String(), v6Port)
			}
			if n.mode == Separate {
				if event.Lease.EnableIPv4 {
					log.Infof("Subnet added: %v via %v", event.Lease.Subnet, v4PeerEndpoint)
					if err := n.dev.addPeer(
						v4PeerEndpoint,
						v4wireguardAttrs.PublicKey,
						[]net.IPNet{*event.Lease.Subnet.ToIPNet()}); err != nil {
						log.Errorf("failed to setup ipv4 peer (%s): %v", v4wireguardAttrs.PublicKey, err)
					}
					netconf, err := n.sm.GetNetworkConfig(ctx)
					if err != nil {
						log.Errorf("could not read network config: %v", err)
					}

					if err := n.dev.addRoute(netconf.Network.ToIPNet()); err != nil {
						log.Errorf("failed to add ipv4 route to (%s): %v", netconf.Network, err)
					}
				}

				if event.Lease.EnableIPv6 {
					log.Infof("Subnet added: %v via %v", event.Lease.IPv6Subnet, v6PeerEndpoint)
					if err := n.v6Dev.addPeer(
						v6PeerEndpoint,
						v6wireguardAttrs.PublicKey,
						[]net.IPNet{*event.Lease.IPv6Subnet.ToIPNet()}); err != nil {
						log.Errorf("failed to setup ipv6 peer (%s): %v", v6wireguardAttrs.PublicKey, err)
					}
					netconf, err := n.sm.GetNetworkConfig(ctx)
					if err != nil {
						log.Errorf("could not read network config: %v", err)
					}

					if err := n.v6Dev.addRoute(netconf.IPv6Network.ToIPNet()); err != nil {
						log.Errorf("failed to add ipv6 route to (%s): %v", netconf.IPv6Network, err)
					}
				}
			} else {
				var publicEndpoint string
				mode := n.mode
				if mode != Ipv4 && mode != Ipv6 { // Auto mode
					mode = n.selectMode(event.Lease.Attrs.PublicIP, event.Lease.Attrs.PublicIPv6)
				}
				switch mode {
				case Ipv4:
					publicEndpoint = v4PeerEndpoint
				case Ipv6:
					publicEndpoint = v6PeerEndpoint
				}

				log.Infof("Subnet(s) added: %v via %v", subnets, publicEndpoint)
				var peers []net.IPNet
				for _, v := range subnets {
					peers = append(peers, *v)
				}
				if err := n.dev.addPeer(
					publicEndpoint,
					wireguardAttrs.PublicKey,
					peers); err != nil {
					log.Errorf("failed to setup peer (%s): %v", v4wireguardAttrs.PublicKey, err)
				}
				netconf, err := n.sm.GetNetworkConfig(ctx)
				if err != nil {
					log.Errorf("could not read network config: %v", err)
				}

				if err := n.dev.addRoute(netconf.Network.ToIPNet()); err != nil {
					log.Errorf("failed to add ipv4 route to (%s): %v", netconf.Network, err)
				}

				if err := n.dev.addRoute(netconf.IPv6Network.ToIPNet()); err != nil {
					log.Errorf("failed to add ipv6 route to (%s): %v", netconf.IPv6Network, err)
				}
			}

		case lease.EventRemoved:

			if event.Lease.Attrs.BackendType != "wireguard" {
				log.Warningf("Ignoring non-wireguard subnet: type=%v", event.Lease.Attrs.BackendType)
				continue
			}

			var wireguardAttrs wireguardLeaseAttrs
			if event.Lease.EnableIPv4 && n.dev != nil {
				log.Info("Subnet removed: ", event.Lease.Subnet)
				if len(event.Lease.Attrs.BackendData) > 0 {
					if err := json.Unmarshal(event.Lease.Attrs.BackendData, &wireguardAttrs); err != nil {
						log.Errorf("failed to unmarshal BackendData: %v", err)
						continue
					}
				}

				if err := n.dev.removePeer(
					wireguardAttrs.PublicKey,
				); err != nil {
					log.Errorf("failed to remove ipv4 peer (%s): %v", wireguardAttrs.PublicKey, err)
				}
			}

			if event.Lease.EnableIPv6 {
				log.Info("Subnet removed: ", event.Lease.IPv6Subnet)
				if len(event.Lease.Attrs.BackendV6Data) > 0 {
					if err := json.Unmarshal(event.Lease.Attrs.BackendV6Data, &wireguardAttrs); err != nil {
						log.Errorf("failed to unmarshal BackendData: %v", err)
						continue
					}
				}

				var err error
				if n.mode == Separate && n.v6Dev != nil {
					err = n.v6Dev.removePeer(wireguardAttrs.PublicKey)
				} else {
					err = n.dev.removePeer(wireguardAttrs.PublicKey)
				}
				if err != nil {
					log.Errorf("failed to remove ipv6 peer (%s): %v", wireguardAttrs.PublicKey, err)
				}
			}
		default:
			log.Error("Internal error: unknown event type: ", int(event.Type))
		}
	}
}
