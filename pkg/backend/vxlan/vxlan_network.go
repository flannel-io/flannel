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
//go:build !windows
// +build !windows

package vxlan

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/flannel-io/flannel/pkg/retry"
	"github.com/flannel-io/flannel/pkg/subnet"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	log "k8s.io/klog/v2"
)

type network struct {
	backend.SimpleNetwork
	dev       *vxlanDevice
	v6Dev     *vxlanDevice
	subnetMgr subnet.Manager
	mtu       int
}

const (
	encapOverhead = 50
)

func newNetwork(subnetMgr subnet.Manager, extIface *backend.ExternalInterface, dev *vxlanDevice, v6Dev *vxlanDevice, _ ip.IP4Net, lease *lease.Lease, mtu int) (*network, error) {
	nw := &network{
		SimpleNetwork: backend.SimpleNetwork{
			SubnetLease: lease,
			ExtIface:    extIface,
		},
		subnetMgr: subnetMgr,
		dev:       dev,
		v6Dev:     v6Dev,
		mtu:       mtu,
	}

	return nw, nil
}

func (nw *network) Run(ctx context.Context) {
	var wg sync.WaitGroup

	log.V(0).Info("watching for new subnet leases")
	leaseEvents := make(chan []lease.Event)
	vxlanMissingChan := make(chan bool, 1) // buffered to avoid blocking

	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, nw.subnetMgr, nw.SubnetLease, leaseEvents)
		log.V(1).Info("WatchLeases exited")
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		nw.watchVXLANDevice(ctx, vxlanMissingChan)
		log.V(1).Info("WatchVXLANDevice exited")
		wg.Done()
	}()

	defer wg.Wait()

	for {
		select {
		case evtBatch, ok := <-leaseEvents:
			if !ok {
				log.Infof("leaseEvents chan closed")
				return
			}
			nw.handleSubnetEvents(evtBatch)

		case _, ok := <-vxlanMissingChan:
			if !ok {
				log.Infof("vxlanMissingChan closed")
				return
			}
			log.Info("vxlan device missing, attempting to recreate...")

			// Offload recreate so this loop doesn’t block handleSubnetEvents
			go func() {
				if err := nw.reCreateVxlan(ctx); err != nil {
					log.Errorf("failed to recreate vxlan: %v", err)
				}
			}()
		}
	}
}

func (nw *network) watchVXLANDevice(ctx context.Context, vxlanMissingChan chan<- bool) {
	log.Info("starting vxlan device watcher")
	if nw.dev == nil {
		log.Error("vxlan device is nil, cannot watch for events")
		return
	}

	updates := make(chan netlink.LinkUpdate)
	done := make(chan struct{})

	if err := netlink.LinkSubscribe(updates, done); err != nil {
		log.Fatalf("failed to subscribe to netlink: %v", err)
	}
	defer close(done)

	name := nw.dev.link.Attrs().Name
	defer close(vxlanMissingChan)
	for {
		select {
		case <-ctx.Done():
			log.Info("stopping vxlan device watcher")
			return

		case update := <-updates:
			if update.Attrs() == nil {
				continue
			}
			// Detect deletion
			if update.Attrs().Name == name && update.Header.Type == unix.RTM_DELLINK {
				log.Infof("Interface %s deleted", name)
				select {
				case vxlanMissingChan <- true:
				default:
					// Skip if signal already queued
				}
			}
		}
	}
}

func (nw *network) reCreateVxlan(ctx context.Context) error {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled, stopping vxlan recreate")
		default:
		}

		extIface, _ := net.InterfaceByName(nw.ExtIface.IfaceName)
		if extIface == nil {
			log.Infof("external interface %s not found, retrying in %s", nw.ExtIface.IfaceName, backoff)
			retryAfterBackoff(&backoff, maxBackoff)
			continue
		}

		config, err := nw.subnetMgr.GetNetworkConfig(ctx)
		if err != nil {
			log.Errorf("failed to get network config: %v", err)
			retryAfterBackoff(&backoff, maxBackoff)
			continue
		}

		cfg, err := parseVXLANConfig(config.Backend, extIface.MTU)
		if err != nil {
			log.Errorf("failed to parse vxlan config: %v", err)
			retryAfterBackoff(&backoff, maxBackoff)
			continue
		}

		var ifaceAddrs, ifaceAddrsV6 []net.IP

		if config.EnableIPv4 {
			ifaceAddrs, err = ip.GetInterfaceIP4Addrs(extIface)
			if err != nil {
				log.Errorf("error getting IPv4 addresses for %s: %v", extIface.Name, err)
				retryAfterBackoff(&backoff, maxBackoff)
				continue
			}
			if len(ifaceAddrs) == 0 {
				log.Warningf("no IPv4 addresses found for interface %s, retrying", extIface.Name)
				retryAfterBackoff(&backoff, maxBackoff)
				continue
			}
		}

		if config.EnableIPv6 {
			ifaceAddrsV6, err = ip.GetInterfaceIP6Addrs(extIface)
			if err != nil {
				log.Errorf("error getting IPv6 addresses for %s: %v", extIface.Name, err)
				retryAfterBackoff(&backoff, maxBackoff)
				continue
			}
			if len(ifaceAddrsV6) == 0 {
				log.Warningf("no IPv6 addresses found for interface %s, retrying", extIface.Name)
				retryAfterBackoff(&backoff, maxBackoff)
				continue
			}
		}

		// Create the VXLAN device
		dev, v6Dev, err := createVXLANDevice(ctx, config, cfg, nw.subnetMgr, extIface.Index, ifaceAddrs[0], ifaceAddrsV6[0])
		if err != nil {
			log.Errorf("failed to create vxlan device: %v", err)
			retryAfterBackoff(&backoff, maxBackoff)
			continue
		}

		if err := configureDeviceIPv4IPv6(dev, v6Dev, nw.SubnetLease, config); err != nil {
			log.Errorf("failed to configure vxlan device: %v", err)
			retryAfterBackoff(&backoff, maxBackoff)
			continue
		}

		nw.dev = dev
		nw.v6Dev = v6Dev
		nw.mtu = dev.link.Attrs().MTU
		log.Infof("VXLAN device %s recreated successfully", dev.link.Attrs().Name)
		return nil
	}
}

func retryAfterBackoff(backoff *time.Duration, maxBackoff time.Duration) {
	time.Sleep(*backoff)
	*backoff = minDuration(*backoff*2, maxBackoff)
}

// helper to cap exponential backoff
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func (nw *network) MTU() int {
	return nw.mtu - encapOverhead
}

type vxlanLeaseAttrs struct {
	VNI     uint32
	VtepMAC hardwareAddr
}

func (nw *network) handleSubnetEvents(batch []lease.Event) {
	for _, event := range batch {
		sn := event.Lease.Subnet
		v6Sn := event.Lease.IPv6Subnet
		attrs := event.Lease.Attrs
		log.Infof("Received Subnet Event with VxLan: %s", attrs.String())
		if attrs.BackendType != "vxlan" {
			log.Warningf("ignoring non-vxlan v4Subnet(%s) v6Subnet(%s): type=%v", sn, v6Sn, attrs.BackendType)
			continue
		}

		var (
			vxlanAttrs, v6VxlanAttrs           vxlanLeaseAttrs
			directRoutingOK, v6DirectRoutingOK bool
			directRoute, v6DirectRoute         netlink.Route
			vxlanRoute, v6VxlanRoute           netlink.Route
		)

		if event.Lease.EnableIPv4 && nw.dev != nil {
			if err := json.Unmarshal(attrs.BackendData, &vxlanAttrs); err != nil {
				log.Error("error decoding subnet lease JSON: ", err)
				continue
			}

			// This route is used when traffic should be vxlan encapsulated
			vxlanRoute = netlink.Route{
				LinkIndex: nw.dev.link.Attrs().Index,
				Scope:     netlink.SCOPE_UNIVERSE,
				Dst:       sn.ToIPNet(),
				Gw:        sn.IP.ToIP(),
			}
			vxlanRoute.SetFlag(syscall.RTNH_F_ONLINK)

			// directRouting is where the remote host is on the same subnet so vxlan isn't required.
			directRoute = netlink.Route{
				Dst: sn.ToIPNet(),
				Gw:  attrs.PublicIP.ToIP(),
			}
			if nw.dev.directRouting {
				if dr, err := ip.DirectRouting(attrs.PublicIP.ToIP()); err != nil {
					log.Error(err)
				} else {
					directRoutingOK = dr
				}
			}
		}

		if event.Lease.EnableIPv6 && nw.v6Dev != nil {
			if err := json.Unmarshal(attrs.BackendV6Data, &v6VxlanAttrs); err != nil {
				log.Error("error decoding v6 subnet lease JSON: ", err)
				continue
			}
			if v6Sn.IP != nil && nw.v6Dev != nil {
				v6VxlanRoute = netlink.Route{
					LinkIndex: nw.v6Dev.link.Attrs().Index,
					Scope:     netlink.SCOPE_UNIVERSE,
					Dst:       v6Sn.ToIPNet(),
					Gw:        v6Sn.IP.ToIP(),
				}
				v6VxlanRoute.SetFlag(syscall.RTNH_F_ONLINK)

				// directRouting is where the remote host is on the same subnet so vxlan isn't required.
				v6DirectRoute = netlink.Route{
					Dst: v6Sn.ToIPNet(),
					Gw:  attrs.PublicIPv6.ToIP(),
				}

				if nw.v6Dev.directRouting {
					if v6Dr, err := ip.DirectRouting(attrs.PublicIPv6.ToIP()); err != nil {
						log.Error(err)
					} else {
						v6DirectRoutingOK = v6Dr
					}
				}
			}
		}

		switch event.Type {
		case lease.EventAdded:
			if event.Lease.EnableIPv4 {
				if directRoutingOK {
					log.V(2).Infof("Adding direct route to subnet: %s PublicIP: %s", sn, attrs.PublicIP)

					if err := retry.Do(func() error {
						return netlink.RouteReplace(&directRoute)
					}); err != nil {
						log.Errorf("Error adding route to %v via %v: %v", sn, attrs.PublicIP, err)
						continue
					}
				} else {
					log.V(2).Infof("adding subnet: %s PublicIP: %s VtepMAC: %s", sn, attrs.PublicIP, net.HardwareAddr(vxlanAttrs.VtepMAC))
					if err := retry.Do(func() error {
						return nw.dev.AddARP(neighbor{IP: sn.IP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)})
					}); err != nil {
						log.Error("AddARP failed: ", err)
						continue
					}

					if err := retry.Do(func() error {
						return nw.dev.AddFDB(neighbor{IP: attrs.PublicIP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)})
					}); err != nil {
						log.Error("AddFDB failed: ", err)

						// Try to clean up the ARP entry then continue
						if err := retry.Do(func() error {
							return nw.dev.DelARP(neighbor{IP: event.Lease.Subnet.IP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)})
						}); err != nil {
							log.Error("DelARP failed: ", err)
						}

						continue
					}

					// Set the route - the kernel would ARP for the Gw IP address if it hadn't already been set above so make sure
					// this is done last.
					if err := retry.Do(func() error {
						return netlink.RouteReplace(&vxlanRoute)
					}); err != nil {
						log.Errorf("failed to add vxlanRoute (%s -> %s): %v", vxlanRoute.Dst, vxlanRoute.Gw, err)

						// Try to clean up both the ARP and FDB entries then continue
						if err := nw.dev.DelARP(neighbor{IP: event.Lease.Subnet.IP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)}); err != nil {
							log.Error("DelARP failed: ", err)
						}

						if err := nw.dev.DelFDB(neighbor{IP: event.Lease.Attrs.PublicIP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)}); err != nil {
							log.Error("DelFDB failed: ", err)
						}

						continue
					}
				}
			}
			if event.Lease.EnableIPv6 {
				if v6DirectRoutingOK {
					log.V(2).Infof("Adding v6 direct route to v6 subnet: %s PublicIPv6: %s", v6Sn, attrs.PublicIPv6)

					if err := retry.Do(func() error {
						return netlink.RouteReplace(&v6DirectRoute)
					}); err != nil {
						log.Errorf("Error adding v6 route to %v via %v: %v", v6Sn, attrs.PublicIPv6, err)
						continue
					}
				} else {
					log.V(2).Infof("adding v6 subnet: %s PublicIPv6: %s VtepMAC: %s", v6Sn, attrs.PublicIPv6, net.HardwareAddr(v6VxlanAttrs.VtepMAC))
					if err := retry.Do(func() error {
						return nw.v6Dev.AddV6ARP(neighbor{IP6: v6Sn.IP, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)})
					}); err != nil {
						log.Error("AddV6ARP failed: ", err)
						continue
					}

					if err := retry.Do(func() error {
						return nw.v6Dev.AddV6FDB(neighbor{IP6: attrs.PublicIPv6, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)})
					}); err != nil {
						log.Error("AddV6FDB failed: ", err)

						// Try to clean up the ARP entry then continue
						if err := nw.v6Dev.DelV6ARP(neighbor{IP6: event.Lease.IPv6Subnet.IP, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)}); err != nil {
							log.Error("DelV6ARP failed: ", err)
						}

						continue
					}

					// Set the route - the kernel would ARP for the Gw IP address if it hadn't already been set above so make sure
					// this is done last.
					if err := retry.Do(func() error {
						return netlink.RouteReplace(&v6VxlanRoute)
					}); err != nil {
						log.Errorf("failed to add v6 vxlanRoute (%s -> %s): %v", v6VxlanRoute.Dst, v6VxlanRoute.Gw, err)

						// Try to clean up both the ARP and FDB entries then continue
						if err := retry.Do(func() error {
							return nw.v6Dev.DelV6ARP(neighbor{IP6: event.Lease.IPv6Subnet.IP, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)})
						}); err != nil {
							log.Error("DelV6ARP failed: ", err)
						}

						if err := retry.Do(func() error {
							return nw.v6Dev.DelV6FDB(neighbor{IP6: event.Lease.Attrs.PublicIPv6, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)})
						}); err != nil {
							log.Error("DelV6FDB failed: ", err)
						}

						continue
					}
				}
			}
		case lease.EventRemoved:
			if event.Lease.EnableIPv4 {
				if directRoutingOK {
					log.V(2).Infof("Removing direct route to subnet: %s PublicIP: %s", sn, attrs.PublicIP)
					if err := retry.Do(func() error {
						return netlink.RouteDel(&directRoute)
					}); err != nil {
						log.Errorf("Error deleting route to %v via %v: %v", sn, attrs.PublicIP, err)
					}
				} else {
					log.V(2).Infof("removing subnet: %s PublicIP: %s VtepMAC: %s", sn, attrs.PublicIP, net.HardwareAddr(vxlanAttrs.VtepMAC))

					// Try to remove all entries - don't bail out if one of them fails.
					if err := retry.Do(func() error {
						return nw.dev.DelARP(neighbor{IP: sn.IP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)})
					}); err != nil {
						log.Error("DelARP failed: ", err)
					}

					if err := retry.Do(func() error {
						return nw.dev.DelFDB(neighbor{IP: attrs.PublicIP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)})
					}); err != nil {
						log.Error("DelFDB failed: ", err)
					}

					if err := retry.Do(func() error {
						return netlink.RouteDel(&vxlanRoute)
					}); err != nil {
						log.Errorf("failed to delete vxlanRoute (%s -> %s): %v", vxlanRoute.Dst, vxlanRoute.Gw, err)
					}
				}
			}
			if event.Lease.EnableIPv6 {
				if v6DirectRoutingOK {
					log.V(2).Infof("Removing v6 direct route to subnet: %s PublicIP: %s", sn, attrs.PublicIPv6)
					if err := retry.Do(func() error {
						return netlink.RouteDel(&directRoute)
					}); err != nil {
						log.Errorf("Error deleting v6 route to %v via %v: %v", v6Sn, attrs.PublicIPv6, err)
					}
				} else {
					log.V(2).Infof("removing v6subnet: %s PublicIPv6: %s VtepMAC: %s", v6Sn, attrs.PublicIPv6, net.HardwareAddr(v6VxlanAttrs.VtepMAC))

					// Try to remove all entries - don't bail out if one of them fails.
					if err := retry.Do(func() error {
						return nw.v6Dev.DelV6ARP(neighbor{IP6: v6Sn.IP, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)})
					}); err != nil {
						log.Error("DelV6ARP failed: ", err)
					}

					if err := retry.Do(func() error {
						return nw.v6Dev.DelV6FDB(neighbor{IP6: attrs.PublicIPv6, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)})
					}); err != nil {
						log.Error("DelV6FDB failed: ", err)
					}

					if err := retry.Do(func() error {
						return netlink.RouteDel(&v6VxlanRoute)
					}); err != nil {
						log.Errorf("failed to delete v6 vxlanRoute (%s -> %s): %v", v6VxlanRoute.Dst, v6VxlanRoute.Gw, err)
					}
				}
			}
		default:
			log.Error("internal error: unknown event type: ", int(event.Type))
		}
	}
}
