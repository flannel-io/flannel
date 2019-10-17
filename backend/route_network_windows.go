// Copyright 2018 flannel authors
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

package backend

import (
	"strings"
	"sync"
	"time"

	"github.com/coreos/flannel/subnet"
	log "github.com/golang/glog"
	"github.com/thxcode/winnet/pkg/ipforward"
	"golang.org/x/net/context"
)

const (
	routeCheckRetries = 10
)

type RouteNetwork struct {
	SimpleNetwork
	Name        string
	BackendType string
	SM          subnet.Manager
	GetRoute    func(lease *subnet.Lease) *ipforward.Route
	Mtu         int
	LinkIndex   int
	routes      []ipforward.Route
}

func (n *RouteNetwork) MTU() int {
	return n.Mtu
}

func (n *RouteNetwork) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.Info("Watching for new subnet leases")
	evts := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, n.SM, n.SubnetLease, evts)
		wg.Done()
	}()

	n.routes = make([]ipforward.Route, 0, 10)
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

func (n *RouteNetwork) handleSubnetEvents(batch []subnet.Event) {
	for _, evt := range batch {
		leaseSubnet := evt.Lease.Subnet
		leaseAttrs := evt.Lease.Attrs
		if !strings.EqualFold(leaseAttrs.BackendType, n.BackendType) {
			log.Warningf("Ignoring non-%v subnet(%v): type=%v", n.BackendType, leaseSubnet, leaseAttrs.BackendType)
			continue
		}

		expectedRoute := n.GetRoute(&evt.Lease)

		switch evt.Type {
		case subnet.EventAdded:
			log.Infof("Subnet added: %v via %v", leaseSubnet, leaseAttrs.PublicIP)

			existingRoutes, _ := ipforward.GetNetRoutes(expectedRoute.LinkIndex, expectedRoute.DestinationSubnet)
			if len(existingRoutes) > 0 {
				existingRoute := existingRoutes[0]
				if existingRoute.Equal(*expectedRoute) {
					continue
				}

				log.Warningf("Replacing existing route %v via %v with %v via %v", leaseSubnet, existingRoute.GatewayAddress, leaseSubnet, leaseAttrs.PublicIP)
				err := ipforward.RemoveNetRoute(existingRoute.LinkIndex, existingRoute.DestinationSubnet, existingRoute.GatewayAddress)
				if err != nil {
					log.Errorf("Error removing route: %v", err)
					continue
				}
			}

			err := ipforward.NewNetRoute(expectedRoute.LinkIndex, expectedRoute.DestinationSubnet, expectedRoute.GatewayAddress)
			if err != nil {
				log.Errorf("Error creating route: %v", err)
				continue
			}

			n.addToRouteList(expectedRoute)

		case subnet.EventRemoved:
			log.Infof("Subnet removed: %v", leaseSubnet)

			existingRoutes, _ := ipforward.GetNetRoutes(expectedRoute.LinkIndex, expectedRoute.DestinationSubnet)
			if len(existingRoutes) > 0 {
				existingRoute := existingRoutes[0]
				if existingRoute.Equal(*expectedRoute) {
					log.Infof("Removing existing route %v via %v", leaseSubnet, existingRoute.GatewayAddress)

					err := ipforward.RemoveNetRoute(existingRoute.LinkIndex, existingRoute.DestinationSubnet, existingRoute.GatewayAddress)
					if err != nil {
						log.Warningf("Error removing route: %v", err)
					}
				}
			}

			n.removeFromRouteList(expectedRoute)

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func (n *RouteNetwork) addToRouteList(newRoute *ipforward.Route) {
	for _, route := range n.routes {
		if route.Equal(*newRoute) {
			return
		}
	}

	n.routes = append(n.routes, *newRoute)
}

func (n *RouteNetwork) removeFromRouteList(oldRoute *ipforward.Route) {
	for index, route := range n.routes {
		if route.Equal(*oldRoute) {
			n.routes = append(n.routes[:index], n.routes[index+1:]...)
			return
		}
	}
}

func (n *RouteNetwork) routeCheck(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(routeCheckRetries * time.Second):
			n.checkSubnetExistInRoutes()
		}
	}
}

func (n *RouteNetwork) checkSubnetExistInRoutes() {
	existingRoutes, err := ipforward.GetNetRoutesAll()
	if err != nil {
		log.Errorf("Error enumerating routes: %v", err)
		return
	}
	for _, expectedRoute := range n.routes {
		exist := false
		for _, existingRoute := range existingRoutes {
			if expectedRoute.Equal(existingRoute) {
				exist = true
				break
			}
		}

		if !exist {
			err := ipforward.NewNetRoute(expectedRoute.LinkIndex, expectedRoute.DestinationSubnet, expectedRoute.GatewayAddress)
			if err != nil {
				log.Warningf("Error recovering route to %v via %v on %v (%v).", expectedRoute.DestinationSubnet, expectedRoute.GatewayAddress, expectedRoute.LinkIndex, err)
				continue
			}
			log.Infof("Recovered route to %v via %v on %v.", expectedRoute.DestinationSubnet, expectedRoute.GatewayAddress, expectedRoute.LinkIndex)
		}
	}
}
