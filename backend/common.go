package backend

import (
	"errors"
	"net"

	"github.com/coreos/rudder/pkg/ip"
)

var ErrInterrupted = errors.New("Interrupted by user")

type Backend interface {
	Init(extIface *net.Interface, extIP net.IP, ipMasq bool) (ip.IP4Net, int, error)
	Run()
	Stop()
	Name() string
}
