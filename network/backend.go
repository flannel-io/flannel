package network

import (
	"fmt"
	"strings"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/backend/alloc"
	"github.com/coreos/flannel/backend/awsvpc"
	"github.com/coreos/flannel/backend/gce"
	"github.com/coreos/flannel/backend/hostgw"
	"github.com/coreos/flannel/backend/udp"
	"github.com/coreos/flannel/backend/vxlan"
	"github.com/coreos/flannel/subnet"
)

var backendMap = map[string]func(sm subnet.Manager, network string, config *subnet.Config) backend.Backend {
	"udp":     udp.New,
	"alloc":   alloc.New,
	"host-gw": hostgw.New,
	"vxlan":   vxlan.New,
	"aws-vpc": awsvpc.New,
	"gce":     gce.New,
}

func newBackend(sm subnet.Manager, network string, config *subnet.Config) (backend.Backend, error) {
	betype := strings.ToLower(config.BackendType)
	befunc, ok := backendMap[betype]
	if !ok {
		return nil, fmt.Errorf("%v: '%v': unknown backend type", network, betype)
	}
	return befunc(sm, network, config), nil
}
