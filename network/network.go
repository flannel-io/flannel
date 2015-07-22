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

type Network struct {
	Name string

	sm     subnet.Manager
	ipMasq bool
	be     backend.Backend
}

func New(sm subnet.Manager, name string, ipMasq bool) *Network {
	return &Network{
		Name:   name,
		sm:     sm,
		ipMasq: ipMasq,
	}
}

func (n *Network) Init(ctx context.Context, iface *net.Interface, iaddr net.IP, eaddr net.IP) *backend.SubnetDef {
	var cfg *subnet.Config
	var be backend.Backend
	var sn *backend.SubnetDef

	steps := []func() error{
		func() (err error) {
			cfg, err = n.sm.GetNetworkConfig(ctx, n.Name)
			if err != nil {
				log.Error("Failed to retrieve network config: ", err)
			}
			return
		},

		func() (err error) {
			be, err = newBackend(n.sm, n.Name, cfg)
			if err != nil {
				log.Error("Failed to create backend: ", err)
			} else {
				n.be = be
			}
			return
		},

		func() (err error) {
			sn, err = be.Init(iface, iaddr, eaddr)
			if err != nil {
				log.Errorf("Failed to initialize network %v (type %v): %v", n.Name, be.Name(), err)
			}
			return
		},

		func() (err error) {
			if n.ipMasq {
				flannelNet := cfg.Network
				if err = setupIPMasq(flannelNet); err != nil {
					log.Errorf("Failed to set up IP Masquerade for network %v: %v", n.Name, err)
				}
			}
			return
		},
	}

	for _, s := range steps {
		for ; ; time.Sleep(time.Second) {
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			err := s()
			if err == nil {
				break
			}
		}
	}

	return sn
}

func (n *Network) Run(ctx context.Context) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		n.be.Run()
		wg.Done()
	}()

	<-ctx.Done()
	n.be.Stop()

	wg.Wait()
}
