package backend

import (
	"net"

	"github.com/coreos/flannel/pkg/ip"
)

type SubnetDef struct {
	Net ip.IP4Net
	MTU int
}

type Backend interface {
	Init(extIface *net.Interface, extIP net.IP) (*SubnetDef, error)
	Run()
	Stop()
	Name() string
}
