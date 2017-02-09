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

package vxlan

import (
	"encoding/json"
	"net"
	"sync"

	log "github.com/golang/glog"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"

	"syscall"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

type network struct {
	backend.SimpleNetwork
	extIface  *backend.ExternalInterface
	dev       *vxlanDevice
	subnetMgr subnet.Manager
}

func newNetwork(subnetMgr subnet.Manager, extIface *backend.ExternalInterface, dev *vxlanDevice, _ ip.IP4Net, lease *subnet.Lease) (*network, error) {
	nw := &network{
		SimpleNetwork: backend.SimpleNetwork{
			SubnetLease: lease,
			ExtIface:    extIface,
		},
		subnetMgr: subnetMgr,
		dev:       dev,
	}

	return nw, nil
}

func (nw *network) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.V(0).Info("watching for new subnet leases")
	events := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, nw.subnetMgr, nw.SubnetLease, events)
		log.V(1).Info("WatchLeases exited")
		wg.Done()
	}()

	defer wg.Wait()

	for {
		select {
		case evtBatch := <-events:
			nw.handleSubnetEvents(evtBatch)

		case <-ctx.Done():
			return
		}
	}
}

func (nw *network) MTU() int {
	return nw.dev.MTU()
}

type vxlanLeaseAttrs struct {
	VtepMAC hardwareAddr
}

func (nw *network) handleSubnetEvents(batch []subnet.Event) {
	for _, event := range batch {
		if event.Lease.Attrs.BackendType != "vxlan" {
			log.Warningf("ignoring non-vxlan subnet(%s): type=%v", event.Lease.Subnet, event.Lease.Attrs.BackendType)
			continue
		}

		var attrs vxlanLeaseAttrs
		if err := json.Unmarshal(event.Lease.Attrs.BackendData, &attrs); err != nil {
			log.Error("error decoding subnet lease JSON: ", err)
			continue
		}

		route := netlink.Route{
			LinkIndex: nw.dev.link.Attrs().Index,
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       event.Lease.Subnet.ToIPNet(),
			Gw:        event.Lease.Subnet.IP.ToIP(),
		}
		route.SetFlag(syscall.RTNH_F_ONLINK)

		switch event.Type {
		case subnet.EventAdded:
			log.V(2).Infof("adding subnet: %s PublicIP: %s VtepMAC: %s", event.Lease.Subnet, event.Lease.Attrs.PublicIP, net.HardwareAddr(attrs.VtepMAC))

			if err := nw.dev.AddARP(neighbor{IP: event.Lease.Subnet.IP, MAC: net.HardwareAddr(attrs.VtepMAC)}); err != nil {
				log.Error("AddARP failed: ", err)
				continue
			}

			if err := nw.dev.AddFDB(neighbor{IP: event.Lease.Attrs.PublicIP, MAC: net.HardwareAddr(attrs.VtepMAC)}); err != nil {
				log.Error("AddFDB failed: ", err)

				// Try to clean up the ARP entry then continue
				if err := nw.dev.DelARP(neighbor{IP: event.Lease.Subnet.IP, MAC: net.HardwareAddr(attrs.VtepMAC)}); err != nil {
					log.Error("DelARP failed: ", err)
				}

				continue
			}

			// Set the route - the kernel would ARP for the Gw IP address if it hadn't already been set above so make sure
			// this is done last.
			if err := netlink.RouteReplace(&route); err != nil {
				log.Errorf("failed to add route (%s -> %s): %v", route.Dst, route.Gw, err)

				// Try to clean up both the ARP and FDB entries then continue
				if err := nw.dev.DelARP(neighbor{IP: event.Lease.Subnet.IP, MAC: net.HardwareAddr(attrs.VtepMAC)}); err != nil {
					log.Error("DelARP failed: ", err)
				}

				if err := nw.dev.DelFDB(neighbor{IP: event.Lease.Attrs.PublicIP, MAC: net.HardwareAddr(attrs.VtepMAC)}); err != nil {
					log.Error("DelFDB failed: ", err)
				}

				continue
			}

		case subnet.EventRemoved:
			log.V(2).Infof("removing subnet: %s PublicIP: %s VtepMAC: %s", event.Lease.Subnet, event.Lease.Attrs.PublicIP, net.HardwareAddr(attrs.VtepMAC))

			// Try to remove all entries - don't bail out if one of them fails.
			if err := nw.dev.DelARP(neighbor{IP: event.Lease.Subnet.IP, MAC: net.HardwareAddr(attrs.VtepMAC)}); err != nil {
				log.Error("DelARP failed: ", err)
			}

			if err := nw.dev.DelFDB(neighbor{IP: event.Lease.Attrs.PublicIP, MAC: net.HardwareAddr(attrs.VtepMAC)}); err != nil {
				log.Error("DelFDB failed: ", err)
			}

			if err := netlink.RouteDel(&route); err != nil {
				log.Errorf("failed to delete route (%s -> %s): %v", route.Dst, route.Gw, err)
			}

		default:
			log.Error("internal error: unknown event type: ", int(event.Type))
		}
	}
}
