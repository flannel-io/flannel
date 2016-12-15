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
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/golang/glog"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

type network struct {
	backend.SimpleNetwork
	name     string
	extIface *backend.ExternalInterface
	dev      *vxlanDevice
	rts      routes
	sm       subnet.Manager
}

func newNetwork(name string, sm subnet.Manager, extIface *backend.ExternalInterface, dev *vxlanDevice, nw ip.IP4Net, l *subnet.Lease) (*network, error) {
	n := &network{
		SimpleNetwork: backend.SimpleNetwork{
			SubnetLease: l,
			ExtIface:    extIface,
		},
		name: name,
		sm:   sm,
		dev:  dev,
	}

	return n, nil
}

func (n *network) Run(ctx context.Context) {
	log.V(0).Info("Watching for L3 misses")
	misses := make(chan *netlink.Neigh, 100)
	// Unfortunately MonitorMisses does not take a cancel channel
	// as there's no wait to interrupt netlink socket recv
	go n.dev.MonitorMisses(misses)

	wg := sync.WaitGroup{}

	log.V(0).Info("Watching for new subnet leases")
	evts := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, n.sm, n.name, n.SubnetLease, evts)
		log.V(1).Info("WatchLeases exited")
		wg.Done()
	}()

	defer wg.Wait()

	select {
	case initialEvtsBatch := <-evts:
		for {
			err := n.handleInitialSubnetEvents(initialEvtsBatch)
			if err == nil {
				break
			}
			log.Error(err, " About to retry")
			time.Sleep(time.Second)
		}

	case <-ctx.Done():
		return
	}

	for {
		select {
		case miss := <-misses:
			n.handleMiss(miss)

		case evtBatch := <-evts:
			n.handleSubnetEvents(evtBatch)

		case <-ctx.Done():
			return
		}
	}
}

func (n *network) MTU() int {
	return n.dev.MTU()
}

type vxlanLeaseAttrs struct {
	VtepMAC hardwareAddr
}

func (n *network) handleSubnetEvents(batch []subnet.Event) {
	for _, evt := range batch {
		switch evt.Type {
		case subnet.EventAdded:
			log.V(1).Info("Subnet added: ", evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "vxlan" {
				log.Warningf("Ignoring non-vxlan subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			var attrs vxlanLeaseAttrs
			if err := json.Unmarshal(evt.Lease.Attrs.BackendData, &attrs); err != nil {
				log.Error("Error decoding subnet lease JSON: ", err)
				continue
			}
			n.rts.set(evt.Lease.Subnet, net.HardwareAddr(attrs.VtepMAC))
			n.dev.AddL2(neigh{IP: evt.Lease.Attrs.PublicIP, MAC: net.HardwareAddr(attrs.VtepMAC)})

		case subnet.EventRemoved:
			log.V(1).Info("Subnet removed: ", evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "vxlan" {
				log.Warningf("Ignoring non-vxlan subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			var attrs vxlanLeaseAttrs
			if err := json.Unmarshal(evt.Lease.Attrs.BackendData, &attrs); err != nil {
				log.Error("Error decoding subnet lease JSON: ", err)
				continue
			}

			if len(attrs.VtepMAC) > 0 {
				n.dev.DelL2(neigh{IP: evt.Lease.Attrs.PublicIP, MAC: net.HardwareAddr(attrs.VtepMAC)})
			}
			n.rts.remove(evt.Lease.Subnet)

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func (n *network) handleInitialSubnetEvents(batch []subnet.Event) error {
	log.V(1).Infof("Handling initial subnet events")
	fdbTable, err := n.dev.GetL2List()
	if err != nil {
		return fmt.Errorf("error fetching L2 table: %v", err)
	}

	// Log the existing VTEP -> Public IP mappings
	for _, fdbEntry := range fdbTable {
		log.V(1).Infof("fdb already populated with: %s %s ", fdbEntry.IP, fdbEntry.HardwareAddr)
	}

	// "marked" events are skipped at the end.
	evtMarker := make([]bool, len(batch))
	leaseAttrsList := make([]vxlanLeaseAttrs, len(batch))
	fdbEntryMarker := make([]bool, len(fdbTable))

	// Run through the events "marking" ones that should be skipped
	for i, evt := range batch {
		if evt.Lease.Attrs.BackendType != "vxlan" {
			log.Warningf("Ignoring non-vxlan subnet(%s): type=%v", evt.Lease.Subnet, evt.Lease.Attrs.BackendType)
			evtMarker[i] = true
			continue
		}

		// Parse the vxlan specific backend data
		if err := json.Unmarshal(evt.Lease.Attrs.BackendData, &leaseAttrsList[i]); err != nil {
			log.Error("Error decoding subnet lease JSON: ", err)
			evtMarker[i] = true
			continue
		}

		// Check the existing VTEP->Public IP mappings.
		// If there's already an entry with the right VTEP and Public IP then the event can be skipped and the FDB entry can be retained
		for j, fdbEntry := range fdbTable {
			if evt.Lease.Attrs.PublicIP.ToIP().Equal(fdbEntry.IP) && bytes.Equal([]byte(leaseAttrsList[i].VtepMAC), []byte(fdbEntry.HardwareAddr)) {
				evtMarker[i] = true
				fdbEntryMarker[j] = true
				break
			}
		}

		// Store off the subnet lease and VTEP
		n.rts.set(evt.Lease.Subnet, net.HardwareAddr(leaseAttrsList[i].VtepMAC))
		log.V(2).Infof("Adding subnet: %s PublicIP: %s VtepMAC: %s", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP, net.HardwareAddr(leaseAttrsList[i].VtepMAC))
	}

	// Loop over the existing FDB entries, deleting any that shouldn't be there
	for j, marker := range fdbEntryMarker {
		if !marker && fdbTable[j].IP != nil {
			err := n.dev.DelL2(neigh{IP: ip.FromIP(fdbTable[j].IP), MAC: fdbTable[j].HardwareAddr})
			if err != nil {
				log.Error("Delete L2 failed: ", err)
			}
		}
	}

	// Loop over the events (skipping marked ones), adding them to the FDB table.
	for i, marker := range evtMarker {
		if !marker {
			err := n.dev.AddL2(neigh{IP: batch[i].Lease.Attrs.PublicIP, MAC: net.HardwareAddr(leaseAttrsList[i].VtepMAC)})
			if err != nil {
				log.Error("Add L2 failed: ", err)
			}
		}
	}
	return nil
}

func (n *network) handleMiss(miss *netlink.Neigh) {
	switch {
	case len(miss.IP) == 0 && len(miss.HardwareAddr) == 0:
		log.V(2).Info("Ignoring nil miss")

	case len(miss.HardwareAddr) == 0:
		n.handleL3Miss(miss)

	default:
		log.V(4).Infof("Ignoring not a miss: %v, %v", miss.HardwareAddr, miss.IP)
	}
}

func (n *network) handleL3Miss(miss *netlink.Neigh) {
	rt := n.rts.findByNetwork(ip.FromIP(miss.IP))
	if rt == nil {
		log.V(0).Infof("L3 miss but route for %v not found", miss.IP)
		return
	}

	if err := n.dev.AddL3(neigh{IP: ip.FromIP(miss.IP), MAC: rt.vtepMAC}); err != nil {
		log.Errorf("AddL3 failed: %v", err)
	} else {
		log.V(2).Infof("L3 miss: AddL3 for %s succeeded", miss.IP)
	}
}
