package hostgw

import (
	"fmt"
	"net"
	"strings"
	"sync"

	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/vishvananda/netlink"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/pkg/task"
	"github.com/coreos/flannel/subnet"
)

type HostgwBackend struct {
	sm       *subnet.SubnetManager
	extIface *net.Interface
	extIP    net.IP
	stop     chan bool
	wg       sync.WaitGroup
}

func New(sm *subnet.SubnetManager) backend.Backend {
	b := &HostgwBackend{
		sm:   sm,
		stop: make(chan bool),
	}
	return b
}

func (rb *HostgwBackend) Init(extIface *net.Interface, extIP net.IP, ipMasq bool) (*backend.SubnetDef, error) {
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

	if ipMasq {
		if err := setupIpMasq(sn, rb.sm.GetConfig().Network); err != nil {
			return nil, fmt.Errorf("Failed to configure ip-masq: %v", err)
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

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func setupIpMasq(localNet ip.IP4Net, overlayNet ip.IP4Net) error {
	ipt, err := ip.NewIPTables()
	if err != nil {
		return err
	}

	err = ipt.ClearChain("nat", "FLANNEL")
	if err != nil {
		return fmt.Errorf("Failed to create/clear FLANNEL chain in NAT table: %v", err)
	}

	rules := [][]string{
		[]string{"FLANNEL", "-s", localNet.String(), "-o", "lo", "-j", "ACCEPT"},
		[]string{"FLANNEL", "-s", localNet.String(), "!", "-d", overlayNet.String(), "-j", "MASQUERADE"},
		[]string{"POSTROUTING", "-s", localNet.String(), "-j", "FLANNEL"},
	}

	for _, args := range rules {
		log.Info("Adding iptables rule: ", strings.Join(args, " "))

		if err := ipt.AppendUnique("nat", args...); err != nil {
			return fmt.Errorf("Failed to insert IP masquerade rule: %v", err)
		}
	}

	return nil
}
