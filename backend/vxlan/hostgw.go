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
	"sync"
	"time"
	"strings"
	"strconv"

	"golang.org/x/net/context"
	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/backend/hostgw"
	"github.com/coreos/flannel/subnet"
	log "github.com/golang/glog"
	"github.com/vishvananda/netlink"
	"github.com/coreos/flannel/pkg/ip"
)

type vxlanGWNetwork struct {
	network
	hostGWNetwork* hostgw.HostGWNetwork
	localNetworkMask uint32
}

type VXLANGWBackend struct {
	VXLANBackend
}

func init() {
	backend.Register("vxlan-over-hostgw", NewHostGw)
}

func getLocalNetworkMask(extIface *backend.ExternalInterface) uint32 {
	var networkLen string = "32"
	var networkMask uint32 = 0xffffffff
	if addrs, err := extIface.Iface.Addrs(); err == nil {
		for _, addr := range addrs {
			 parts := strings.Split(addr.String(), "/")
			 if len(parts) == 2 {
				 if parts[0] != extIface.ExtAddr.String() {
					 continue
				 }
				 networkLen = parts[1]
				 break
			 }
		 }
	}
	i, err := strconv.Atoi(networkLen)
	if err == nil && i <= 32 {
		networkMask = (networkMask << uint32(32 - i))
	}
	return networkMask
}

func NewHostGw(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	backend := &VXLANGWBackend{
		VXLANBackend: VXLANBackend{
			subnetMgr: sm,
			extIface:  extIface,
			backendType: "vxlan-over-hostgw",
		},
	}
	return backend, nil
}

func (be *VXLANGWBackend) RegisterNetwork(ctx context.Context, config *subnet.Config) (backend.Network, error) {
	vxlan, err := be.VXLANBackend.RegisterNetwork(ctx, config)
	if err != nil {
		return nil, err
	}
	vxlanNetwork := vxlan.(*network)
	n := &vxlanGWNetwork {
		network: *vxlanNetwork,
		hostGWNetwork: hostgw.NewHostGWNetwork(
			vxlanNetwork.subnetMgr,
			vxlanNetwork.ExtIface,
			vxlanNetwork.SimpleNetwork.SubnetLease,
		),
		localNetworkMask: getLocalNetworkMask(vxlanNetwork.ExtIface),
	}
	return n, err
}

func (nw *vxlanGWNetwork) MTU() int {
	return nw.network.MTU()
}

func (nw *vxlanGWNetwork) Lease() *subnet.Lease {
	return nw.network.Lease()
}

func (nw *vxlanGWNetwork) filterLocalSubnetEvents(batch []subnet.Event) []subnet.Event{
	local_ip := ip.FromIP(nw.network.ExtIface.ExtAddr)
	filteredEvent := make([]subnet.Event, 0)
	for _, event := range batch {
		if event.Type != subnet.EventAdded && event.Type != subnet.EventRemoved {
			continue
		}
		remote_ip := event.Lease.Attrs.PublicIP
		if ((uint32)(local_ip ^ remote_ip) & nw.localNetworkMask ) != 0{
			continue
		}
		filteredEvent = append(filteredEvent, event)
	}
	return filteredEvent

}

func (nw *vxlanGWNetwork) Run(ctx context.Context) {
	misses := make(chan *netlink.Neigh, 100)
	// Unfortunately MonitorMisses does not take a cancel channel
	// as there's no wait to interrupt netlink socket recv
	go nw.dev.MonitorMisses(misses)

	wg := sync.WaitGroup{}

	events := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, nw.subnetMgr, nw.SubnetLease, events)
		wg.Done()
	}()
	// add host gw watches
	nw.hostGWNetwork.SetupRun(ctx, wg)

	defer wg.Wait()

	select {
	case initialEventsBatch := <-events:
		for {
			err := nw.handleInitialSubnetEvents(initialEventsBatch)
			if err == nil {
				// host gw check
				filtered_events := nw.filterLocalSubnetEvents(initialEventsBatch)
				nw.hostGWNetwork.HandleSubnetEvents(filtered_events)
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
			nw.handleMiss(miss)

		case evtBatch := <-events:
			nw.handleSubnetEvents(evtBatch)
			// host gw check
			filtered_events := nw.filterLocalSubnetEvents(evtBatch)
			nw.hostGWNetwork.HandleSubnetEvents(filtered_events)

		case <-ctx.Done():
			return
		}
	}
}
