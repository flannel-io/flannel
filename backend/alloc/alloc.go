package alloc

import (
	"fmt"
	"net"

	"github.com/coreos/rudder/backend"
	"github.com/coreos/rudder/pkg/ip"
	"github.com/coreos/rudder/pkg/task"
	"github.com/coreos/rudder/subnet"
)


type AllocBackend struct {
	sm   *subnet.SubnetManager
	stop chan bool
}

func New(sm *subnet.SubnetManager) backend.Backend {
	return &AllocBackend{
		sm: sm,
		stop: make(chan bool),
	}
}

func (m *AllocBackend) Init(extIface *net.Interface, extIP net.IP, ipMasq bool) (*backend.SubnetDef, error) {
	attrs := subnet.BaseAttrs{
		PublicIP: ip.FromIP(extIP),
	}

	sn, err := m.sm.AcquireLease(ip.FromIP(extIP), &attrs, m.stop)
	if err != nil {
		if err == task.ErrCanceled {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to acquire lease: %v", err)
		}
	}

	return &backend.SubnetDef{
		Net: sn,
		MTU: extIface.MTU,
	}, nil
}

func (m *AllocBackend) Run() {
	m.sm.LeaseRenewer(m.stop)
}

func (m *AllocBackend) Stop() {
	close(m.stop)
}

func (m *AllocBackend) Name() string {
	return "allocation"
}
