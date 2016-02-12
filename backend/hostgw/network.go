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
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/vishvananda/netlink"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/subnet"
)

type network struct {
	name      string
	extIface  *backend.ExternalInterface
	linkIndex int
	rl        []netlink.Route
	lease     *subnet.Lease
	sm        subnet.Manager
	cfg       hostGwBackendConfig
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

	n.rl = make([]netlink.Route, 0, 10)
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

func (n *network) determineRoute() (netlink.Route, error) {
	route := netlink.Route{
		Dst: n.lease.Subnet.ToIPNet(),
	}

	switch {
	case n.cfg.DefaultGw && n.cfg.Gw != nil:
		log.Warning("defaultGw and Gw set at the same time, preferring DefaultGw..")
		fallthrough

	// determine and use DefaultGw as route
	case n.cfg.DefaultGw:
		routesToDstIP, err := netlink.RouteGet(n.lease.Subnet.IP.ToIP())
		if err != nil {
			return route, err
		}
		routeToDstIP := routesToDstIP[0]
		route.LinkIndex = routeToDstIP.LinkIndex
		route.Gw = routeToDstIP.Gw
		route.Src = routeToDstIP.Src

	// use requested Gw
	case n.cfg.Gw != nil:
		route.Gw = n.cfg.Gw

		routesToGwIP, err := netlink.RouteGet(route.Gw)
		if err != nil {
			return route, err
		}
		routeToGwIP := routesToGwIP[0]
		route.LinkIndex = routeToGwIP.LinkIndex
		route.Src = routeToGwIP.Src

	// default behavior. use external IP/interface as route
	case !n.cfg.DefaultGw && n.cfg.Gw == nil:
		route.Gw = n.lease.Attrs.PublicIP.ToIP()
		route.LinkIndex = n.linkIndex

	default:
		return route, fmt.Errorf("matched none of the expectec gateway configurations")
	}
	log.Infof("Route: %+v", route)
	return route, nil
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

			log.Info(n.cfg)

			route, err := n.determineRoute()
			if err != nil {
				log.Errorf("Error determining route to %v", evt.Lease.Subnet, err)
				continue
			}
			if err := netlink.RouteAdd(&route); err != nil {
				log.Errorf("Error adding route to %v via %v: %v", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP, err)
				continue
			}
			n.addToRouteList(route)

		case subnet.EventRemoved:
			log.Info("Subnet removed: ", evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "host-gw" {
				log.Warningf("Ignoring non-host-gw subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			route, err := n.determineRoute()
			if err != nil {
				log.Errorf("Error determining route to %v", evt.Lease.Subnet, err)
				continue
			}
			if err := netlink.RouteDel(&route); err != nil {
				log.Errorf("Error deleting route to %v: %v", evt.Lease.Subnet, err)
				continue
			}
			n.removeFromRouteList(route)

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func (n *network) addToRouteList(route netlink.Route) {
	n.rl = append(n.rl, route)
}

func (n *network) removeFromRouteList(route netlink.Route) {
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
	routeList, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err == nil {
		for _, route := range n.rl {
			exist := false
			for _, r := range routeList {
				if r.Dst == nil {
					continue
				}
				if routeEqual(r, route) {
					exist = true
					break
				}
			}
			if !exist {
				if err := netlink.RouteAdd(&route); err != nil {
					if nerr, ok := err.(net.Error); !ok {
						log.Errorf("Error recovering route to %v: %v, %v", route.Dst, route.Gw, nerr)
					}
					continue
				} else {
					log.Infof("Route recovered %v : %v", route.Dst, route.Gw)
				}
			}
		}
	}
}

func routeEqual(x, y netlink.Route) bool {
	if x.Dst.IP.Equal(y.Dst.IP) && x.Gw.Equal(y.Gw) && bytes.Equal(x.Dst.Mask, y.Dst.Mask) {
		return true
	}
	return false
}
