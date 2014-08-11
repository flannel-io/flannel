package backend

import (
	"github.com/coreos-inc/kolach/pkg"
)

type ReadyFunc func(sn pkg.IP4Net, mtu int)
