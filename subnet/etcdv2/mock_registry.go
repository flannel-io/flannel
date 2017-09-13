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

package etcdv2

import (
	"fmt"
	"sync"
	"time"

	etcd "github.com/coreos/etcd/client"
	"github.com/jonboulle/clockwork"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/pkg/ip"
	. "github.com/coreos/flannel/subnet"
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

func (n *netwk) subnetEventsChan(sn ip.IP4Net) chan event {
	n.mux.Lock()
	c, ok := n.subnetEvents[sn]
	if !ok {
		c = make(chan event, 10)
		n.subnetEvents[sn] = c
	}
	n.mux.Unlock()
	return c
}

type event struct {
	evt   Event
	index uint64
}

type MockSubnetRegistry struct {
	mux     sync.Mutex
	network *netwk
	index   uint64
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

func (msr *MockSubnetRegistry) getSubnets(ctx context.Context) ([]Lease, uint64, error) {
	//msr.mux.Lock()
	//defer msr.mux.Unlock()

	subs := make([]Lease, len(msr.network.subnets))
	copy(subs, msr.network.subnets)
	return subs, msr.index, nil
}

func (msr *MockSubnetRegistry) getSubnet(ctx context.Context, sn ip.IP4Net) (*Lease, uint64, error) {
	msr.mux.Lock()
	defer msr.mux.Unlock()

	for _, l := range msr.network.subnets {
		if l.Subnet.Equal(sn) {
			return &l, msr.index, nil
		}
	}
	return nil, msr.index, fmt.Errorf("subnet %s not found", sn)
}

func (msr *MockSubnetRegistry) createSubnet(ctx context.Context, sn ip.IP4Net, attrs *LeaseAttrs, ttl time.Duration) (time.Time, error) {
	msr.mux.Lock()
	defer msr.mux.Unlock()

	// check for existing
	if _, _, err := msr.network.findSubnet(sn); err == nil {
		return time.Time{}, etcd.Error{
			Code:  etcd.ErrorCodeNodeExist,
			Index: msr.index,
		}
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

func (msr *MockSubnetRegistry) updateSubnet(ctx context.Context, sn ip.IP4Net, attrs *LeaseAttrs, ttl time.Duration, asof uint64) (time.Time, error) {
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

func (msr *MockSubnetRegistry) deleteSubnet(ctx context.Context, sn ip.IP4Net) error {
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

func (msr *MockSubnetRegistry) watchSubnets(ctx context.Context, since uint64) (Event, uint64, error) {
	for {
		msr.mux.Lock()
		index := msr.index
		msr.mux.Unlock()

		if since < index {
			return Event{}, 0, etcd.Error{
				Code:    etcd.ErrorCodeEventIndexCleared,
				Cause:   "out of date",
				Message: "cursor is out of date",
				Index:   index,
			}
		}

		select {
		case <-ctx.Done():
			return Event{}, 0, ctx.Err()

		case e := <-msr.network.subnetsEvents:
			if e.index > since {
				return e.evt, e.index, nil
			}
		}
	}
}

func (msr *MockSubnetRegistry) watchSubnet(ctx context.Context, since uint64, sn ip.IP4Net) (Event, uint64, error) {
	for {
		msr.mux.Lock()
		index := msr.index
		msr.mux.Unlock()

		if since < index {
			return Event{}, msr.index, etcd.Error{
				Code:    etcd.ErrorCodeEventIndexCleared,
				Cause:   "out of date",
				Message: "cursor is out of date",
				Index:   index,
			}
		}

		select {
		case <-ctx.Done():
			return Event{}, index, ctx.Err()

		case e := <-msr.network.subnetEventsChan(sn):
			if e.index > since {
				return e.evt, index, nil
			}
		}
	}
}

func (msr *MockSubnetRegistry) expireSubnet(network string, sn ip.IP4Net) {
	if sub, i, err := msr.network.findSubnet(sn); err == nil {
		msr.index += 1
		msr.network.subnets[i] = msr.network.subnets[len(msr.network.subnets)-1]
		msr.network.subnets = msr.network.subnets[:len(msr.network.subnets)-1]
		sub.Asof = msr.index
		msr.network.sendSubnetEvent(sn, event{
			Event{
				Type:  EventRemoved,
				Lease: sub,
			}, msr.index,
		})
	}
}

func (msr *MockSubnetRegistry) getNetwork(ctx context.Context) (*netwk, error) {
	return msr.network, nil
}

func (n *netwk) findSubnet(sn ip.IP4Net) (Lease, int, error) {
	for i, sub := range n.subnets {
		if sub.Subnet.Equal(sn) {
			return sub, i, nil
		}
	}
	return Lease{}, 0, fmt.Errorf("subnet not found")
}
