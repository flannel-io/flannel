// Copyright 2015 flannel authors
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
// +build windows

package hostgw

import (
	"golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/subnet"

	netroute "github.com/rakelkar/gonetsh/netroute"
)

type network struct {
	name      string
	extIface  *backend.ExternalInterface
	linkIndex int
	rl        []netroute.Route
	lease     *subnet.Lease
	sm        subnet.Manager
}

func (n *network) Lease() *subnet.Lease {
	return n.lease
}

func (n *network) MTU() int {
	return n.extIface.Iface.MTU
}

func (n *network) Run(ctx context.Context) {

}
