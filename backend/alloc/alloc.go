package alloc

import (
	"fmt"
	"net"

	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

type AllocBackend struct {
	sm      subnet.Manager
	network string
	lease   *subnet.Lease
	ctx     context.Context
	cancel  context.CancelFunc
}

func New(sm subnet.Manager, network string) backend.Backend {
	ctx, cancel := context.WithCancel(context.Background())

	return &AllocBackend{
		sm:      sm,
		network: network,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (m *AllocBackend) Init(extIface *net.Interface, extIaddr net.IP, extEaddr net.IP) (*backend.SubnetDef, error) {
	attrs := subnet.LeaseAttrs{
		PublicIP: ip.FromIP(extEaddr),
	}

	l, err := m.sm.AcquireLease(m.ctx, m.network, &attrs)
	switch err {
	case nil:
		m.lease = l
		return &backend.SubnetDef{
			Net: l.Subnet,
			MTU: extIface.MTU,
		}, nil

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}
}

func (m *AllocBackend) Run() {
	subnet.LeaseRenewer(m.ctx, m.sm, m.network, m.lease)
}

func (m *AllocBackend) Stop() {
	m.cancel()
}

func (m *AllocBackend) Name() string {
	return "allocation"
}
