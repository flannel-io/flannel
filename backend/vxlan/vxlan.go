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

package vxlan

import (
	"encoding/json"
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
	defaultVNI = 1
)

type VXLANBackend struct {
	sm     *subnet.SubnetManager
	rawCfg json.RawMessage
	cfg    struct {
		VNI  int
		Port int
	}
	dev  *vxlanDevice
	stop chan bool
	wg   sync.WaitGroup
	rts  routes
}

func New(sm *subnet.SubnetManager, config json.RawMessage) backend.Backend {
	vb := &VXLANBackend{
		sm:     sm,
		rawCfg: config,
		stop:   make(chan bool),
	}
	vb.cfg.VNI = defaultVNI

	return vb
}

func newSubnetAttrs(pubIP net.IP, mac net.HardwareAddr) (*subnet.LeaseAttrs, error) {
	data, err := json.Marshal(&vxlanLeaseAttrs{hardwareAddr(mac)})
	if err != nil {
		return nil, err
	}

	return &subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(pubIP),
		BackendType: "vxlan",
		BackendData: json.RawMessage(data),
	}, nil
}

func (vb *VXLANBackend) Init(extIface *net.Interface, extIP net.IP) (*backend.SubnetDef, error) {
	// Parse our configuration
	if len(vb.rawCfg) > 0 {
		if err := json.Unmarshal(vb.rawCfg, &vb.cfg); err != nil {
			return nil, fmt.Errorf("error decoding UDP backend config: %v", err)
		}
	}

	devAttrs := vxlanDeviceAttrs{
		vni:       uint32(vb.cfg.VNI),
		name:      fmt.Sprintf("flannel.%v", vb.cfg.VNI),
		vtepIndex: extIface.Index,
		vtepAddr:  extIP,
		vtepPort:  vb.cfg.Port,
	}

	var err error
	for {
		vb.dev, err = newVXLANDevice(&devAttrs)
		if err == nil {
			break
		} else {
			log.Error("VXLAN init: ", err)
			log.Info("Retrying in 1 second...")

			// wait 1 sec before retrying
			time.Sleep(1 * time.Second)
		}
	}

	sa, err := newSubnetAttrs(extIP, vb.dev.MACAddr())
	if err != nil {
		return nil, err
	}

	sn, err := vb.sm.AcquireLease(sa, vb.stop)
	if err != nil {
		if err == task.ErrCanceled {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to acquire lease: %v", err)
		}
	}

	// vxlan's subnet is that of the whole overlay network (e.g. /16)
	// and not that of the individual host (e.g. /24)
	vxlanNet := ip.IP4Net{
		IP:        sn.IP,
		PrefixLen: vb.sm.GetConfig().Network.PrefixLen,
	}
	if err = vb.dev.Configure(vxlanNet); err != nil {
		return nil, err
	}

	return &backend.SubnetDef{
		Net: sn,
		MTU: vb.dev.MTU(),
	}, nil
}

func (vb *VXLANBackend) Run() {
	vb.wg.Add(1)
	go func() {
		vb.sm.LeaseRenewer(vb.stop)
		vb.wg.Done()
	}()

	log.Info("Watching for L2/L3 misses")
	misses := make(chan *netlink.Neigh, 100)
	// Unfortunately MonitorMisses does not take a cancel channel
	// as there's no wait to interrupt netlink socket recv
	go vb.dev.MonitorMisses(misses)

	log.Info("Watching for new subnet leases")
	evts := make(chan subnet.EventBatch)
	vb.wg.Add(1)
	go func() {
		vb.sm.WatchLeases(evts, vb.stop)
		vb.wg.Done()
	}()

	defer vb.wg.Wait()

	for {
		select {
		case miss := <-misses:
			vb.handleMiss(miss)

		case evtBatch := <-evts:
			vb.handleSubnetEvents(evtBatch)

		case <-vb.stop:
			return
		}
	}
}

func (vb *VXLANBackend) Stop() {
	close(vb.stop)
}

func (vb *VXLANBackend) Name() string {
	return "VXLAN"
}

// So we can make it JSON (un)marshalable
type hardwareAddr net.HardwareAddr

func (hw hardwareAddr) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", net.HardwareAddr(hw))), nil
}

func (hw *hardwareAddr) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("error parsing hardware addr")
	}

	b = b[1 : len(b)-1]

	mac, err := net.ParseMAC(string(b))
	if err != nil {
		return err
	}

	*hw = hardwareAddr(mac)
	return nil
}

type vxlanLeaseAttrs struct {
	VtepMAC hardwareAddr
}

func (vb *VXLANBackend) handleSubnetEvents(batch subnet.EventBatch) {
	for _, evt := range batch {
		switch evt.Type {
		case subnet.SubnetAdded:
			log.Info("Subnet added: ", evt.Lease.Network)

			if evt.Lease.Attrs.BackendType != "vxlan" {
				log.Warningf("Ignoring non-vxlan subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			var attrs vxlanLeaseAttrs
			if err := json.Unmarshal(evt.Lease.Attrs.BackendData, &attrs); err != nil {
				log.Error("Error decoding subnet lease JSON: ", err)
				continue
			}

			vb.rts.set(evt.Lease.Network, evt.Lease.Attrs.PublicIP.ToIP(), net.HardwareAddr(attrs.VtepMAC))

		case subnet.SubnetRemoved:
			log.Info("Subnet removed: ", evt.Lease.Network)

			vb.rts.remove(evt.Lease.Network)

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
				vb.dev.DelL2(net.HardwareAddr(attrs.VtepMAC), evt.Lease.Attrs.PublicIP.ToIP())
			}

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func (vb *VXLANBackend) handleMiss(miss *netlink.Neigh) {
	switch {
	case len(miss.IP) == 0 && len(miss.HardwareAddr) == 0:
		log.Info("Ignoring nil miss")

	case len(miss.IP) == 0:
		vb.handleL2Miss(miss)

	case len(miss.HardwareAddr) == 0:
		vb.handleL3Miss(miss)

	default:
		log.Infof("Ignoring not a miss: %v, %v", miss.HardwareAddr, miss.IP)
	}
}

func (vb *VXLANBackend) handleL2Miss(miss *netlink.Neigh) {
	log.Infof("L2 miss: %v", miss.HardwareAddr)

	rt := vb.rts.findByVtepMAC(miss.HardwareAddr)
	if rt == nil {
		log.Infof("Route for %v not found", miss.HardwareAddr)
		return
	}

	if err := vb.dev.AddL2(miss.HardwareAddr, rt.vtepIP); err != nil {
		log.Errorf("AddL2 failed: %v", err)
	} else {
		log.Info("AddL2 succeeded")
	}
}

func (vb *VXLANBackend) handleL3Miss(miss *netlink.Neigh) {
	log.Infof("L3 miss: %v", miss.IP)

	rt := vb.rts.findByNetwork(ip.FromIP(miss.IP))
	if rt == nil {
		log.Infof("Route for %v not found", miss.IP)
		return
	}

	if err := vb.dev.AddL3(miss.IP, rt.vtepMAC); err != nil {
		log.Errorf("AddL3 failed: %v", err)
	} else {
		log.Info("AddL3 succeeded")
	}
}
