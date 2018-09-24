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
	log "github.com/golang/glog"
	"golang.org/x/net/context"
	"sync"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/subnet"

	"github.com/coreos/flannel/pkg/ip"
	"strings"
)

type network struct {
	backend.SimpleNetwork
	dev       *vxlanDevice
	subnetMgr subnet.Manager
}

const (
	encapOverhead = 50
)

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

	log.V(0).Info("Watching for new subnet leases")
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
	return nw.ExtIface.Iface.MTU - encapOverhead
}

func (nw *network) handleSubnetEvents(batch []subnet.Event) {
	for _, event := range batch {
		leaseSubnet := event.Lease.Subnet
		leaseAttrs := event.Lease.Attrs
		if !strings.EqualFold(leaseAttrs.BackendType, "vxlan") {
			log.Warningf("ignoring non-vxlan subnet(%v): type=%v", leaseSubnet, leaseAttrs.BackendType)
			continue
		}

		publicIP := leaseAttrs.PublicIP.String()
		remoteIP := leaseSubnet.IP + 2
		lastIP := leaseSubnet.Next().IP - 1

		switch event.Type {
		case subnet.EventAdded:
			for ; remoteIP < lastIP; remoteIP++ {
				n := &neighbor{
					IP:                remoteIP,
					MAC:               nw.dev.ConjureMac(remoteIP),
					ManagementAddress: publicIP,
				}

				log.V(2).Infof("adding subnet: %v publicIP: %s vtepMAC: %s", leaseSubnet, n.ManagementAddress, n.MAC)
				if err := nw.dev.AddEndpoint(n); err != nil {
					log.Error(err)
				}
			}
		case subnet.EventRemoved:
			for ; remoteIP < lastIP; remoteIP++ {
				n := &neighbor{
					IP:                remoteIP,
					MAC:               nw.dev.ConjureMac(remoteIP),
					ManagementAddress: publicIP,
				}

				log.V(2).Infof("removing subnet: %v publicIP: %s vtepMAC: %s", leaseSubnet, n.ManagementAddress, n.MAC)
				if err := nw.dev.DelEndpoint(n); err != nil {
					log.Error(err)
				}
			}
		default:
			log.Error("internal error: unknown event type: ", int(event.Type))
		}
	}
}
