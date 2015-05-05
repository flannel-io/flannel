// Copyright 2015 CoreOS, Inc.
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

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/pkg/task"
	"github.com/coreos/flannel/subnet"
)

const (
	routeCheckRetries = 10
)

type HostgwBackend struct {
	sm       *subnet.SubnetManager
	extIface *net.Interface
	extIP    net.IP
	stop     chan bool
	wg       sync.WaitGroup
	rl       []netlink.Route
}

func New(sm *subnet.SubnetManager) backend.Backend {
	b := &HostgwBackend{
		sm:   sm,
		stop: make(chan bool),
	}
	return b
}

func (rb *HostgwBackend) Init(extIface *net.Interface, extIP net.IP) (*backend.SubnetDef, error) {
	rb.extIface = extIface
	rb.extIP = extIP

	attrs := subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(extIP),
		BackendType: "host-gw",
	}

	sn, err := rb.sm.AcquireLease(&attrs, rb.stop)
	if err != nil {
		if err == task.ErrCanceled {
			return nil, err
		} else {
			return nil, fmt.Errorf("Failed to acquire lease: %v", err)
		}
	}

	/* NB: docker will create the local route to `sn` */

	return &backend.SubnetDef{
		Net: sn,
		MTU: extIface.MTU,
	}, nil
}

func (rb *HostgwBackend) Run() {
	rb.wg.Add(1)
	go func() {
		rb.sm.LeaseRenewer(rb.stop)
		rb.wg.Done()
	}()

	log.Info("Watching for new subnet leases")
	evts := make(chan subnet.EventBatch)
	rb.wg.Add(1)
	go func() {
		rb.sm.WatchLeases(evts, rb.stop)
		rb.wg.Done()
	}()

	rb.rl = make([]netlink.Route, 0, 10)
	rb.wg.Add(1)
	go func() {
		rb.routeCheck(rb.stop)
		rb.wg.Done()
	}()

	defer rb.wg.Wait()

	for {
		select {
		case evtBatch := <-evts:
			rb.handleSubnetEvents(evtBatch)

		case <-rb.stop:
			return
		}
	}
}

func (rb *HostgwBackend) Stop() {
	close(rb.stop)
}

func (rb *HostgwBackend) Name() string {
	return "host-gw"
}

func (rb *HostgwBackend) handleSubnetEvents(batch subnet.EventBatch) {
	for _, evt := range batch {
		switch evt.Type {
		case subnet.SubnetAdded:
			log.Infof("Subnet added: %v via %v", evt.Lease.Network, evt.Lease.Attrs.PublicIP)

			if evt.Lease.Attrs.BackendType != "host-gw" {
				log.Warningf("Ignoring non-host-gw subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			route := netlink.Route{
				Dst:       evt.Lease.Network.ToIPNet(),
				Gw:        evt.Lease.Attrs.PublicIP.ToIP(),
				LinkIndex: rb.extIface.Index,
			}
			if err := netlink.RouteAdd(&route); err != nil {
				log.Errorf("Error adding route to %v via %v: %v", evt.Lease.Network, evt.Lease.Attrs.PublicIP, err)
				continue
			}
			rb.addToRouteList(route)

		case subnet.SubnetRemoved:
			log.Info("Subnet removed: ", evt.Lease.Network)

			if evt.Lease.Attrs.BackendType != "host-gw" {
				log.Warningf("Ignoring non-host-gw subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			route := netlink.Route{
				Dst:       evt.Lease.Network.ToIPNet(),
				Gw:        evt.Lease.Attrs.PublicIP.ToIP(),
				LinkIndex: rb.extIface.Index,
			}
			if err := netlink.RouteDel(&route); err != nil {
				log.Errorf("Error deleting route to %v: %v", evt.Lease.Network, err)
				continue
			}
			rb.removeFromRouteList(route)

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func (rb *HostgwBackend) addToRouteList(route netlink.Route) {
	rb.rl = append(rb.rl, route)
}

func (rb *HostgwBackend) removeFromRouteList(route netlink.Route) {
	for index, r := range rb.rl {
		if routeEqual(r, route) {
			rb.rl = append(rb.rl[:index], rb.rl[index+1:]...)
			return
		}
	}
}

func (rb *HostgwBackend) routeCheck(cancel chan bool) {
	for {
		select {
		case <-cancel:
			return
		case <-time.After(routeCheckRetries * time.Second):
			rb.checkSubnetExistInRoutes()
		}
	}
}

func (rb *HostgwBackend) checkSubnetExistInRoutes() {
	routeList, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err == nil {
		for _, route := range rb.rl {
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
