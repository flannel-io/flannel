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

func (m *AllocBackend) Init(extIface *net.Interface, extIP net.IP, ipMasq bool) (ip.IP4Net, int, error) {
	attrs := subnet.BaseAttrs{
		PublicIP: ip.FromIP(extIP),
	}

	sn, err := m.sm.AcquireLease(ip.FromIP(extIP), &attrs, m.stop)
	if err != nil {
		if err == task.ErrCanceled {
			return ip.IP4Net{}, 0, err
		} else {
			return ip.IP4Net{}, 0, fmt.Errorf("Failed to acquire lease: %v", err)
		}
	}

	return sn, extIface.MTU, nil
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
