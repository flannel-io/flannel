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
	"bytes"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/flannel/pkg/routes"
	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/subnet"
)

type network struct {
	name     string
	extIface *backend.ExternalInterface
	rl       []routes.Route
	lease    *subnet.Lease
	sm       subnet.Manager
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
		subnet.WatchLeases(ctx, n.sm, n.name, n.lease, evts)
		wg.Done()
	}()

	n.rl = make([]routes.Route, 0, 10)
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
	for _, evt := range batch {
		switch evt.Type {
		case subnet.EventAdded:
			log.Infof("Subnet added: %v via %v", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP)

			if evt.Lease.Attrs.BackendType != "host-gw" {
				log.Warningf("Ignoring non-host-gw subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			route := routes.Route{
				Destination: evt.Lease.Subnet.ToIPNet(),
				Gateway:     evt.Lease.Attrs.PublicIP.ToIP(),
				LinkIndex:   n.extIface.Iface.Index,
			}

			// Check if route exists before attempting to add it
			routeList, err := routes.RouteListFiltered(routes.FAMILY_V4, &routes.Route{
				Destination: route.Destination,
			}, routes.RT_FILTER_DST)
			if err != nil {
				log.Warningf("Unable to list routes: %v", err)
			}
			//   Check match on Dst for match on Gw
			if len(routeList) > 0 && !routeList[0].Gateway.Equal(route.Gateway) {
				// Same Dst different Gw. Remove it, correct route will be added below.
				log.Warningf("Replacing existing route to %v via %v with %v via %v.", evt.Lease.Subnet, routeList[0].Gateway, evt.Lease.Subnet, evt.Lease.Attrs.PublicIP)
				if err := routes.DeleteRoute(&route); err != nil {
					log.Errorf("Error deleting route to %v: %v", evt.Lease.Subnet, err)
					continue
				}
			}
			if len(routeList) > 0 && routeList[0].Gateway.Equal(route.Gateway) {
				// Same Dst and same Gw, keep it and do not attempt to add it.
				log.Infof("Route to %v via %v already exists, skipping.", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP)
			} else if err := routes.AddRoute(&route); err != nil {

				if err := routes.AddRoute(&route); err != nil {
					errno, ok := err.(syscall.Errno)
					// The Windows errno for "The object already exists" is 0x1392
					if ok && errno == syscall.EEXIST || errno == 0x1392 {
						log.Infof("Route to %v via %v already exists", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP)
						continue
					}

					log.Errorf("Error adding route to %v via %v: %v", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP, err)
					continue
				}

				n.addToRouteList(route)
			}
		case subnet.EventRemoved:
			log.Info("Subnet removed: ", evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "host-gw" {
				log.Warningf("Ignoring non-host-gw subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			route := routes.Route{
				Destination: evt.Lease.Subnet.ToIPNet(),
				Gateway:     evt.Lease.Attrs.PublicIP.ToIP(),
				LinkIndex:   n.extIface.Iface.Index,
			}
			if err := routes.DeleteRoute(&route); err != nil {
				errno, ok := err.(syscall.Errno)
				if ok && errno == syscall.EEXIST {
					log.Infof("Route to %v does not exist", evt.Lease.Subnet)
					continue
				}

				log.Errorf("Error deleting route to %v: %v", evt.Lease.Subnet, err)
				continue
			}
			n.removeFromRouteList(route)

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func (n *network) addToRouteList(route routes.Route) {
	n.rl = append(n.rl, route)
}

func (n *network) removeFromRouteList(route routes.Route) {
	for index, r := range n.rl {
		if routeEqual(r, route) {
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
	routeList, err := routes.RouteList()
	if err == nil {
		for _, route := range n.rl {
			exist := false
			for _, r := range routeList {
				if r.Destination == nil {
					continue
				}
				if routeEqual(r, route) {
					exist = true
					break
				}
			}
			if !exist {
				if err := routes.AddRoute(&route); err != nil {
					if nerr, ok := err.(net.Error); !ok {
						log.Errorf("Error recovering route to %s: %s, %v", route.Destination.IP, route.Gateway, nerr)
					}
					continue
				} else {
					log.Infof("Route recovered %s : %s", route.Destination.IP, route.Gateway)
				}
			}
		}
	}
}

func routeEqual(x, y routes.Route) bool {
	if x.Destination.IP.Equal(y.Destination.IP) && x.Gateway.Equal(y.Gateway) && bytes.Equal(x.Destination.Mask, y.Destination.Mask) {
		return true
	}
	return false
}
