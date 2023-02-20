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

package etcd

import (
	"fmt"
	"sync"
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	. "github.com/flannel-io/flannel/pkg/subnet"
	"github.com/jonboulle/clockwork"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"golang.org/x/net/context"
	log "k8s.io/klog"
)

var clock clockwork.Clock = clockwork.NewRealClock()

type netwk struct {
	config        string
	subnets       []Lease
	subnetsEvents chan event

	mux          sync.Mutex
	subnetEvents map[ip.IP4Net]chan event
}

func (n *netwk) sendSubnetEvent(sn ip.IP4Net, e event) {
	log.Infof("sendSubnetEvent: sn=[ %s ], e=[ %v ]", sn, e)
	n.subnetsEvents <- e

	n.mux.Lock()
	c, ok := n.subnetEvents[sn]
	if !ok {
		c = make(chan event, 10)
		n.subnetEvents[sn] = c
	}
	n.mux.Unlock()
	c <- e
}

type event struct {
	evt   Event
	index int64
}

type MockSubnetRegistry struct {
	mux     sync.Mutex
	network *netwk
	index   int64
}

func NewMockRegistry(config string, initialSubnets []Lease) *MockSubnetRegistry {
	msr := &MockSubnetRegistry{
		index: 1000,
		network: &netwk{
			config:        config,
			subnets:       initialSubnets,
			subnetsEvents: make(chan event, 1000),
			subnetEvents:  make(map[ip.IP4Net]chan event)},
	}

	return msr
}

func (msr *MockSubnetRegistry) getNetworkConfig(ctx context.Context) (string, error) {
	return msr.network.config, nil
}

func (msr *MockSubnetRegistry) setConfig(config string) error {
	msr.network.config = config
	return nil
}

func (msr *MockSubnetRegistry) getSubnets(ctx context.Context) ([]Lease, int64, error) {
	msr.mux.Lock()
	defer msr.mux.Unlock()

	subs := make([]Lease, len(msr.network.subnets))
	copy(subs, msr.network.subnets)
	return subs, msr.index, nil
}

// TODO ignores ipv6
func (msr *MockSubnetRegistry) getSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) (*Lease, int64, error) {
	msr.mux.Lock()
	defer msr.mux.Unlock()

	for _, l := range msr.network.subnets {
		if l.Subnet.Equal(sn) {
			return &l, msr.index, nil
		}
	}
	return nil, msr.index, fmt.Errorf("subnet %s not found", sn)
}

// TOODO ignores ipv6
func (msr *MockSubnetRegistry) createSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *LeaseAttrs, ttl time.Duration) (time.Time, error) {
	msr.mux.Lock()
	defer msr.mux.Unlock()

	// check for existing
	if _, _, err := msr.network.findSubnet(sn); err == nil {
		return time.Time{}, rpctypes.ErrGRPCKeyNotFound
	}

	msr.index += 1

	exp := time.Time{}
	if ttl != 0 {
		exp = clock.Now().Add(ttl)
	}

	l := Lease{
		Subnet:     sn,
		Attrs:      *attrs,
		Expiration: exp,
		Asof:       msr.index,
	}
	msr.network.subnets = append(msr.network.subnets, l)

	evt := Event{
		Type:  EventAdded,
		Lease: l,
	}

	msr.network.sendSubnetEvent(sn, event{evt, msr.index})

	return exp, nil
}

// TODO ignores ipv6
func (msr *MockSubnetRegistry) updateSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *LeaseAttrs, ttl time.Duration, asof int64) (time.Time, error) {
	msr.mux.Lock()
	defer msr.mux.Unlock()

	msr.index += 1

	exp := time.Time{}
	if ttl != 0 {
		exp = clock.Now().Add(ttl)
	}

	sub, i, err := msr.network.findSubnet(sn)
	if err != nil {
		return time.Time{}, err
	}

	sub.Attrs = *attrs
	sub.Asof = msr.index
	sub.Expiration = exp
	msr.network.subnets[i] = sub
	msr.network.sendSubnetEvent(sn, event{
		Event{
			Type:  EventAdded,
			Lease: sub,
		}, msr.index,
	})

	return sub.Expiration, nil
}

func (msr *MockSubnetRegistry) deleteSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) error {
	msr.mux.Lock()
	defer msr.mux.Unlock()

	msr.index += 1

	sub, i, err := msr.network.findSubnet(sn)
	if err != nil {
		return err
	}

	msr.network.subnets[i] = msr.network.subnets[len(msr.network.subnets)-1]
	msr.network.subnets = msr.network.subnets[:len(msr.network.subnets)-1]
	sub.Asof = msr.index
	msr.network.sendSubnetEvent(sn, event{
		Event{
			Type:  EventRemoved,
			Lease: sub,
		}, msr.index,
	})

	return nil
}

func (msr *MockSubnetRegistry) watchSubnets(ctx context.Context, leaseWatchChan chan []LeaseWatchResult, since int64) error {
	log.Infof("watchSubnets started with since= [ %d]", since)
	for {
		msr.mux.Lock()
		index := msr.index
		msr.mux.Unlock()

		if since < index {
			return rpctypes.ErrGRPCCompacted
		}

		select {
		case <-ctx.Done():
			close(leaseWatchChan)
			return ctx.Err()

		case e := <-msr.network.subnetsEvents:
			if e.index > since {
				leaseWatchChan <- []LeaseWatchResult{
					{Events: []Event{e.evt},
						Cursor: e.index}}
			}
		}
	}
}

// TODO ignores ip6
func (msr *MockSubnetRegistry) watchSubnet(ctx context.Context, since int64, sn ip.IP4Net, sn6 ip.IP6Net, leaseWatchChan chan []LeaseWatchResult) error {
	return errUnimplemented
}

func (msr *MockSubnetRegistry) leasesWatchReset(ctx context.Context) (LeaseWatchResult, error) {
	wr := LeaseWatchResult{}
	leases, index, err := msr.getSubnets(ctx)
	if err != nil {
		return wr, fmt.Errorf("failed to retrieve subnet leases: %v", err)
	}

	wr.Cursor = watchCursor{index}
	wr.Snapshot = leases
	return wr, nil
}

func (n *netwk) findSubnet(sn ip.IP4Net) (Lease, int, error) {
	for i, sub := range n.subnets {
		if sub.Subnet.Equal(sn) {
			return sub, i, nil
		}
	}
	return Lease{}, 0, fmt.Errorf("subnet not found")
}
