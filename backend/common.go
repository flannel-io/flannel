package backend

import (
	"github.com/coreos-inc/rudder/pkg"
)

type ReadyFunc func(sn pkg.IP4Net, mtu int)
