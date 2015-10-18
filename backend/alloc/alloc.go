package alloc

import (
	"fmt"

	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

func init() {
	backend.Register("alloc", New)
}

type AllocBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := AllocBackend{
		sm:       sm,
		extIface: extIface,
	}
	return &be, nil
}

func (_ *AllocBackend) Run(ctx context.Context) {
	<-ctx.Done()
}

func (be *AllocBackend) RegisterNetwork(ctx context.Context, network string, config *subnet.Config) (backend.Network, error) {
	attrs := subnet.LeaseAttrs{
		PublicIP: ip.FromIP(be.extIface.ExtAddr),
	}

	l, err := be.sm.AcquireLease(ctx, network, &attrs)
	switch err {
	case nil:
		return &backend.SimpleNetwork{
			SubnetLease: l,
			ExtIface:    be.extIface,
		}, nil

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}
}
