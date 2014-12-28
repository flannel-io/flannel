package vxlan

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
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

func (vb *VXLANBackend) Init(extIface *net.Interface, extIP net.IP, ipMasq bool) (*backend.SubnetDef, error) {
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
	vb.dev, err = newVXLANDevice(&devAttrs)
	if err != nil {
		return nil, err
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

	return &backend.SubnetDef{sn, vb.dev.MTU()}, nil
}

func (vb *VXLANBackend) Run() {
	vb.wg.Add(1)
	go func() {
		vb.sm.LeaseRenewer(vb.stop)
		vb.wg.Done()
	}()

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

			vb.dev.AddL2(neigh{IP: evt.Lease.Attrs.PublicIP, MAC: net.HardwareAddr(attrs.VtepMAC)})
			vb.dev.AddL3(neigh{IP: evt.Lease.Network.IP, MAC: net.HardwareAddr(attrs.VtepMAC)})
			vb.dev.AddRoute(evt.Lease.Network)

		case subnet.SubnetRemoved:
			log.Info("Subnet removed: ", evt.Lease.Network)

			if evt.Lease.Attrs.BackendType != "vxlan" {
				log.Warningf("Ignoring non-vxlan subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			var attrs vxlanLeaseAttrs
			if err := json.Unmarshal(evt.Lease.Attrs.BackendData, &attrs); err != nil {
				log.Error("Error decoding subnet lease JSON: ", err)
				continue
			}

			vb.dev.DelRoute(evt.Lease.Network)
			if len(attrs.VtepMAC) > 0 {
				vb.dev.DelL2(neigh{IP: evt.Lease.Attrs.PublicIP, MAC: net.HardwareAddr(attrs.VtepMAC)})
				vb.dev.DelL3(neigh{IP: evt.Lease.Network.IP, MAC: net.HardwareAddr(attrs.VtepMAC)})
			}

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}
