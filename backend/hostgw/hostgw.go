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
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

const (
	routeCheckRetries = 10
)

type HostgwBackend struct {
	sm       subnet.Manager
	network  string
	lease    *subnet.Lease
	extIface *net.Interface
	extIaddr net.IP
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	rl       []netlink.Route
}

func New(sm subnet.Manager, network string) backend.Backend {
	ctx, cancel := context.WithCancel(context.Background())

	b := &HostgwBackend{
		sm:      sm,
		network: network,
		ctx:     ctx,
		cancel:  cancel,
	}
	return b
}

func (rb *HostgwBackend) Init(extIface *net.Interface, extIaddr net.IP, extEaddr net.IP) (*backend.SubnetDef, error) {
	rb.extIface = extIface
	rb.extIaddr = extIaddr

	if !extIaddr.Equal(extEaddr) {
		return nil, fmt.Errorf("your PublicIP differs from interface IP, meaning that probably you're on a NAT, which is not supported by host-gw backend")
	}

	attrs := subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(extIaddr),
		BackendType: "host-gw",
	}

	l, err := rb.sm.AcquireLease(rb.ctx, rb.network, &attrs)
	switch err {
	case nil:
		rb.lease = l

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	/* NB: docker will create the local route to `sn` */

	return &backend.SubnetDef{
		Net: l.Subnet,
		MTU: extIface.MTU,
	}, nil
}

func (rb *HostgwBackend) Run() {
	rb.wg.Add(1)
	go func() {
		subnet.LeaseRenewer(rb.ctx, rb.sm, rb.network, rb.lease)
		rb.wg.Done()
	}()

	log.Info("Watching for new subnet leases")
	evts := make(chan []subnet.Event)
	rb.wg.Add(1)
	go func() {
		subnet.WatchLeases(rb.ctx, rb.sm, rb.network, rb.lease, evts)
		rb.wg.Done()
	}()

	rb.rl = make([]netlink.Route, 0, 10)
	rb.wg.Add(1)
	go func() {
		rb.routeCheck(rb.ctx)
		rb.wg.Done()
	}()

	defer rb.wg.Wait()

	for {
		select {
		case evtBatch := <-evts:
			rb.handleSubnetEvents(evtBatch)

		case <-rb.ctx.Done():
			return
		}
	}
}

func (rb *HostgwBackend) Stop() {
	rb.cancel()
}

func (rb *HostgwBackend) Name() string {
	return "host-gw"
}

func (rb *HostgwBackend) handleSubnetEvents(batch []subnet.Event) {
	for _, evt := range batch {
		switch evt.Type {
		case subnet.EventAdded:
			log.Infof("Subnet added: %v via %v", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP)

			if evt.Lease.Attrs.BackendType != "host-gw" {
				log.Warningf("Ignoring non-host-gw subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			route := netlink.Route{
				Dst:       evt.Lease.Subnet.ToIPNet(),
				Gw:        evt.Lease.Attrs.PublicIP.ToIP(),
				LinkIndex: rb.extIface.Index,
			}
			if err := netlink.RouteAdd(&route); err != nil {
				log.Errorf("Error adding route to %v via %v: %v", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP, err)
				continue
			}
			rb.addToRouteList(route)

		case subnet.EventRemoved:
			log.Info("Subnet removed: ", evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "host-gw" {
				log.Warningf("Ignoring non-host-gw subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			route := netlink.Route{
				Dst:       evt.Lease.Subnet.ToIPNet(),
				Gw:        evt.Lease.Attrs.PublicIP.ToIP(),
				LinkIndex: rb.extIface.Index,
			}
			if err := netlink.RouteDel(&route); err != nil {
				log.Errorf("Error deleting route to %v: %v", evt.Lease.Subnet, err)
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

func (rb *HostgwBackend) routeCheck(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
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
