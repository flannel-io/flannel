// +build windows

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

package hostgw

import (
	"sync"
	"time"

	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/subnet"

	netroute "github.com/rakelkar/gonetsh/netroute"
)

type network struct {
	name      string
	extIface  *backend.ExternalInterface
	linkIndex int
	rl        []netroute.Route
	lease     *subnet.Lease
	sm        subnet.Manager
}

func (n *network) Lease() *subnet.Lease {
	return n.lease
}

func (n *network) MTU() int {
	return n.extIface.Iface.MTU
}

func (n *network) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.Info("Watching for new subnet leases")
	evts := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, n.sm, n.lease, evts)
		wg.Done()
	}()

	n.rl = make([]netroute.Route, 0, 10)
	wg.Add(1)
	go func() {
		n.routeCheck(ctx)
		wg.Done()
	}()

	defer wg.Wait()

	for {
		select {
		case evtBatch := <-evts:
			n.handleSubnetEvents(evtBatch)

		case <-ctx.Done():
			return
		}
	}
}
func (n *network) handleSubnetEvents(batch []subnet.Event) {
	nr := netroute.New()
	defer nr.Exit()

	for _, evt := range batch {
		switch evt.Type {
		case subnet.EventAdded:
			log.Infof("Subnet added: %v via %v", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP)

			if evt.Lease.Attrs.BackendType != "host-gw" {
				log.Warningf("Ignoring non-host-gw subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			route := netroute.Route{
				DestinationSubnet: evt.Lease.Subnet.ToIPNet(),
				GatewayAddress:    evt.Lease.Attrs.PublicIP.ToIP(),
				LinkIndex:         n.linkIndex,
			}

			existingRoutes, _ := nr.GetNetRoutes(route.LinkIndex, route.DestinationSubnet)

			if existingRoutes != nil && len(existingRoutes) > 0 {
				if existingRoutes[0].Equal(route) {
					continue
				}

				log.Warningf("Replacing existing route to %v via %v with %v via %v.", evt.Lease.Subnet, existingRoutes[0].GatewayAddress, evt.Lease.Subnet, evt.Lease.Attrs.PublicIP)
				err := nr.RemoveNetRoute(route.LinkIndex, route.DestinationSubnet, existingRoutes[0].GatewayAddress)
				if err != nil {
					log.Errorf("Error removing route: %v", err)
					continue
				}
			}

			err := nr.NewNetRoute(route.LinkIndex, route.DestinationSubnet, route.GatewayAddress)
			if err != nil {
				log.Errorf("Error creating route: %v", err)
			}

			n.addToRouteList(route)

		case subnet.EventRemoved:
			log.Info("Subnet removed: ", evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "host-gw" {
				log.Warningf("Ignoring non-host-gw subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			route := netroute.Route{
				DestinationSubnet: evt.Lease.Subnet.ToIPNet(),
				GatewayAddress:    evt.Lease.Attrs.PublicIP.ToIP(),
				LinkIndex:         n.linkIndex,
			}

			existingRoutes, _ := nr.GetNetRoutes(route.LinkIndex, route.DestinationSubnet)

			if existingRoutes != nil {
				err := nr.RemoveNetRoute(route.LinkIndex, route.DestinationSubnet, route.GatewayAddress)
				if err != nil {
					log.Errorf("Error removing route: %v", err)
				}
			}

			n.removeFromRouteList(route)

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func (n *network) addToRouteList(route netroute.Route) {
	for _, r := range n.rl {
		if r.Equal(route) {
			return
		}
	}
	n.rl = append(n.rl, route)
}

func (n *network) removeFromRouteList(route netroute.Route) {
	for index, r := range n.rl {
		if r.Equal(route) {
			n.rl = append(n.rl[:index], n.rl[index+1:]...)
			return
		}
	}
}

func (n *network) routeCheck(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(routeCheckRetries * time.Second):
			n.checkSubnetExistInRoutes()
		}
	}
}

func (n *network) checkSubnetExistInRoutes() {
	nr := netroute.New()
	defer nr.Exit()

	currentRoutes, err := nr.GetNetRoutesAll()
	if err != nil {
		log.Errorf("Error enumerating routes", err)
		return
	}
	for _, r := range n.rl {
		exist := false
		for _, currentRoute := range currentRoutes {
			if r.Equal(currentRoute) {
				exist = true
				break
			}
		}

		if !exist {
			err := nr.NewNetRoute(r.LinkIndex, r.DestinationSubnet, r.GatewayAddress)
			if err != nil {
				log.Errorf("Error recovering route to %v via %v on %v (%v).", r.DestinationSubnet, r.GatewayAddress, r.LinkIndex, err)
				continue
			}
			log.Errorf("Recovered route to %v via %v on %v.", r.DestinationSubnet, r.GatewayAddress, r.LinkIndex)
		}
	}
}