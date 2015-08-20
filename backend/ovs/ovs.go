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

package ovs

import (
	"encoding/json"
	"fmt"
	"sync"

	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

const (
	CLUSTER_NETWORK_NAME = "~~flannel-ovs-cluster-network~~"
)

type OVSBackend struct {
	sm         subnet.Manager
	clusterNet *network
	dev        *ovsDevice
	extIface   *backend.ExternalInterface
	wg         sync.WaitGroup
}

func init() {
	backend.Register("ovs", New)
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := &OVSBackend{
		sm:       sm,
		extIface: extIface,
	}

	var err error
	if be.dev, err = newOVSDevice(); err != nil {
		return nil, err
	}

	return be, nil
}

func (ovsb *OVSBackend) RegisterNetwork(ctx context.Context, netname string, config *subnet.Config) (backend.Network, error) {
	if netname != CLUSTER_NETWORK_NAME {
		return nil, fmt.Errorf("this plugin only handles the %s network", CLUSTER_NETWORK_NAME)
	}

	servicesNetwork, err := parseClusterNetworkConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cluster network %s config: %v", CLUSTER_NETWORK_NAME, err)
	}

	// Acquire a lease for this node
	sa := &subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(ovsb.extIface.ExtAddr),
		BackendType: "ovs",
		BackendData: json.RawMessage(""),
	}
	lease, err := ovsb.sm.AcquireLease(ctx, CLUSTER_NETWORK_NAME, sa)
	switch err {
	case nil:
		break

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire cluster network lease: %v", err)
	}
	log.Infof("ClusterNetwork %s found; node subnet is %s", config.Network, lease.Subnet)

	if err := ovsb.dev.nodeSetup(config.Network, lease.Subnet, servicesNetwork); err != nil {
		return nil, fmt.Errorf("failed to set up local node: %v", err)
	}

	ovsb.clusterNet, err = newNetwork(netname, config, ovsb.extIface.Iface.MTU, lease, ovsb)
	if err != nil {
		return nil, err
	}

	return ovsb.clusterNet, nil
}

func parseClusterNetworkConfig(config *subnet.Config) (*ip.IP4Net, error) {
	if len(config.Backend) == 0 {
		return nil, fmt.Errorf("no ServicesNetwork specified")
	}

	var data struct {
		ServicesNetwork ip.IP4Net
	}

	if err := json.Unmarshal(config.Backend, &data); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %v", err)
	}
	return &data.ServicesNetwork, nil
}

func (ovsb *OVSBackend) watchNodeLeases(ctx context.Context) {
	evts := make(chan []subnet.Event)
	ovsb.wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, ovsb.sm, CLUSTER_NETWORK_NAME, nil, evts)
		log.Infof("WatchNodeLeases exited")
		ovsb.wg.Done()
	}()

	log.Errorf("rcrcrc - watching %s\n", CLUSTER_NETWORK_NAME)
	initialEvtsBatch := <-evts
	ovsb.handleNodeEvents(initialEvtsBatch)

	for {
		select {
		case evtBatch := <-evts:
			ovsb.handleNodeEvents(evtBatch)

		case <-ctx.Done():
			return
		}
	}
}

func (ovsb *OVSBackend) removeNetwork(network *network) {
	ovsb.clusterNet = nil
}

func (ovsb *OVSBackend) Run(ctx context.Context) {
	ovsb.wg.Add(1)
	go func() {
		ovsb.watchNodeLeases(ctx)
		ovsb.wg.Done()
	}()

	<-ctx.Done()

	ovsb.wg.Wait()
}

func (ovsb *OVSBackend) handleNodeEvents(batch []subnet.Event) {
	for _, evt := range batch {
		switch evt.Type {
		case subnet.EventAdded:
			log.Infof("Node added: %s => %s", evt.Lease.Attrs.PublicIP, evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "ovs" {
				log.Warningf("Ignoring non-ovs subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			if ovsb.extIface.IfaceAddr.String() != evt.Lease.Attrs.PublicIP.String() {
				ovsb.dev.AddRemoteSubnet(evt.Lease.Subnet.ToIPNet(), evt.Lease.Attrs.PublicIP.ToIP())
			}

		case subnet.EventRemoved:
			log.Infof("Node removed: %s => %s", evt.Lease.Attrs.PublicIP, evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "ovs" {
				log.Warningf("Ignoring non-ovs subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			if ovsb.extIface.IfaceAddr.String() != evt.Lease.Attrs.PublicIP.String() {
				ovsb.dev.RemoveRemoteSubnet(evt.Lease.Subnet.ToIPNet(), evt.Lease.Attrs.PublicIP.ToIP())
			}

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}
