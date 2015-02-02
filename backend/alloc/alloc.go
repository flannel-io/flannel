package alloc

import (
	"fmt"
	"net"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/pkg/task"
	"github.com/coreos/flannel/subnet"
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

func (m *AllocBackend) Init(extIface *net.Interface, extIP net.IP) (*backend.SubnetDef, error) {
	attrs := subnet.LeaseAttrs{
		PublicIP: ip.FromIP(extIP),
	}

	sn, err := m.sm.AcquireLease(&attrs, m.stop)
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
