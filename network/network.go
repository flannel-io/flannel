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

package network

import (
	"net"
	"sync"
	"time"

	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/subnet"
)

const (
	renewMargin = time.Hour
)

type Network struct {
	Name       string
	Config     *subnet.Config
	ctx        context.Context
	cancelFunc context.CancelFunc

	sm     subnet.Manager
	ipMasq bool
	be     backend.Backend
	lease  *subnet.Lease
}

func NewNetwork(sm subnet.Manager, name string, ipMasq bool) *Network {
	return &Network{
		Name:   name,
		sm:     sm,
		ipMasq: ipMasq,
	}
}

func (n *Network) Init(ctx context.Context, iface *net.Interface, iaddr net.IP, eaddr net.IP) (*backend.SubnetDef, error) {
	var sn *backend.SubnetDef

	n.ctx, n.cancelFunc = context.WithCancel(ctx)

	type step struct {
		desc string
		fn   func() error
	}

	steps := []step{
		step{"retrieve network config", func() (err error) {
			n.Config, err = n.sm.GetNetworkConfig(n.ctx, n.Name)
			return
		}},

		step{"create and initialize network", func() (err error) {
			n.be, err = newBackend(n.sm, n.Config.BackendType, iface, iaddr, eaddr)
			return
		}},

		step{"register network", func() (err error) {
			sn, err = n.be.RegisterNetwork(n.ctx, n.Name, n.Config)
			if err == nil {
				n.lease = sn.Lease
			}
			return
		}},

		step{"set up IP Masquerade", func() (err error) {
			if n.ipMasq {
				err = setupIPMasq(n.Config.Network)
			}
			return
		}},
	}

	for _, s := range steps {
	RetryFor:
		for {
			err := s.fn()
			switch err {
			case nil:
				break RetryFor
			case context.Canceled:
				return nil, err
			default:
				log.Errorf("%q: failed to %v: %v", n.Name, s.desc, err)
			}

			select {
			case <-time.After(time.Second):

			case <-n.ctx.Done():
				return nil, n.ctx.Err()
			}
		}
	}

	return sn, nil
}

func (n *Network) Run() {
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		n.be.Run(n.ctx)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		subnet.LeaseRenewer(n.ctx, n.sm, n.Name, n.lease)
		wg.Done()
	}()

	defer func() {
		n.be.UnregisterNetwork(n.ctx, n.Name)
		if n.ipMasq {
			if err := teardownIPMasq(n.Config.Network); err != nil {
				log.Errorf("Failed to tear down IP Masquerade for network %v: %v", n.Name, err)
			}
		}
	}()

	<-n.ctx.Done()
	wg.Wait()
}

func (n *Network) Cancel() {
	n.cancelFunc()
}
