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
}

func New(sm subnet.Manager, network string) backend.Backend {
	return &AllocBackend{
		sm:      sm,
		network: network,
	}
}

func (m *AllocBackend) Init(ctx context.Context, extIface *net.Interface, extIaddr net.IP, extEaddr net.IP) (*backend.SubnetDef, error) {
	attrs := subnet.LeaseAttrs{
		PublicIP: ip.FromIP(extEaddr),
	}

	l, err := m.sm.AcquireLease(ctx, m.network, &attrs)
	switch err {
	case nil:
		m.lease = l
		return &backend.SubnetDef{
			Lease: l,
			MTU:   extIface.MTU,
		}, nil

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}
}

func (m *AllocBackend) Run(ctx context.Context) {
}
