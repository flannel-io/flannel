package backend

import (
	"net"

	"github.com/coreos/rudder/pkg/ip"
)

type Backend interface {
	Init(extIface *net.Interface, extIP net.IP, ipMasq bool) (ip.IP4Net, int, error)
	Run()
	Stop()
	Name() string
}
