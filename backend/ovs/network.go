// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovs

import (
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/flannel/subnet"
)

type network struct {
	name  string
	lease *subnet.Lease
	mtu   int
	be    *OVSBackend
}

func newNetwork(netname string, config *subnet.Config, mtu int, lease *subnet.Lease, be *OVSBackend) (*network, error) {
	return &network{
		name:  netname,
		lease: lease,
		mtu:   mtu,
		be:    be,
	}, nil
}

func (n *network) Lease() *subnet.Lease {
	return n.lease
}

func (n *network) MTU() int {
	return n.mtu
}

func (n *network) Run(ctx context.Context) {
	<-ctx.Done()
	n.be.removeNetwork(n)
}
