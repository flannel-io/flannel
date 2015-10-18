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
	"fmt"
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

var backends map[string]backend.Backend

type Network struct {
	Name   string
	Config *subnet.Config

	ctx        context.Context
	cancelFunc context.CancelFunc
	sm         subnet.Manager
	bm         backend.Manager
	ipMasq     bool
	bn         backend.Network
}

func NewNetwork(ctx context.Context, sm subnet.Manager, bm backend.Manager, name string, ipMasq bool) *Network {
	ctx, cf := context.WithCancel(ctx)

	return &Network{
		Name:       name,
		sm:         sm,
		bm:         bm,
		ipMasq:     ipMasq,
		ctx:        ctx,
		cancelFunc: cf,
	}
}

func wrapError(desc string, err error) error {
	if err == context.Canceled {
		return err
	}
	return fmt.Errorf("failed to %v: %v", desc, err)
}

func (n *Network) init() error {
	var err error

	n.Config, err = n.sm.GetNetworkConfig(n.ctx, n.Name)
	if err != nil {
		return wrapError("retrieve network config", err)
	}

	be, err := n.bm.GetBackend(n.Config.BackendType)
	if err != nil {
		return wrapError("create and initialize network", err)
	}

	n.bn, err = be.RegisterNetwork(n.ctx, n.Name, n.Config)
	if err != nil {
		return wrapError("register network", err)
	}

	if n.ipMasq {
		err = setupIPMasq(n.Config.Network)
		if err != nil {
			return wrapError("set up IP Masquerade", err)
		}
	}

	return nil
}

func (n *Network) Run(extIface *backend.ExternalInterface, inited func(bn backend.Network)) {
	wg := sync.WaitGroup{}

For:
	for {
		err := n.init()
		switch err {
		case nil:
			break For
		case context.Canceled:
			return
		default:
			log.Error(err)
			select {
			case <-n.ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}

	inited(n.bn)

	wg.Add(1)
	go func() {
		n.bn.Run(n.ctx)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		subnet.LeaseRenewer(n.ctx, n.sm, n.Name, n.bn.Lease())
		wg.Done()
	}()

	defer func() {
		if n.ipMasq {
			if err := teardownIPMasq(n.Config.Network); err != nil {
				log.Errorf("Failed to tear down IP Masquerade for network %v: %v", n.Name, err)
			}
		}
	}()

	wg.Wait()
}

func (n *Network) Cancel() {
	n.cancelFunc()
}
