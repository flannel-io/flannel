package backend

import (
	"github.com/coreos/rudder/pkg/ip"
)

type ReadyFunc func(sn ip.IP4Net, mtu int)
