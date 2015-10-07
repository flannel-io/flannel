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

package subnet

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	etcd "github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/etcd/client"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/pkg/ip"
)

const (
	registerRetries = 10
	subnetTTL       = 24 * time.Hour
)

type LocalManager struct {
	registry Registry
}

type watchCursor struct {
	index uint64
}

func (c watchCursor) String() string {
	return strconv.FormatUint(c.index, 10)
}

func NewLocalManager(config *EtcdConfig) (Manager, error) {
	r, err := newEtcdSubnetRegistry(config)
	if err != nil {
		return nil, err
	}
	return newLocalManager(r), nil
}

func newLocalManager(r Registry) Manager {
	return &LocalManager{
		registry: r,
	}
}

func (m *LocalManager) GetNetworkConfig(ctx context.Context, network string) (*Config, error) {
	cfg, err := m.registry.getNetworkConfig(ctx, network)
	if err != nil {
		return nil, err
	}

	return ParseConfig(cfg)
}

func (m *LocalManager) AcquireLease(ctx context.Context, network string, attrs *LeaseAttrs) (*Lease, error) {
	config, err := m.GetNetworkConfig(ctx, network)
	if err != nil {
		return nil, err
	}

	for i := 0; i < registerRetries; i++ {
		l, err := m.tryAcquireLease(ctx, network, config, attrs.PublicIP, attrs)
		switch {
		case err != nil:
			return nil, err
		case l != nil:
			return l, nil
		}
	}

	return nil, errors.New("Max retries reached trying to acquire a subnet")
}

func findLeaseByIP(leases []Lease, pubIP ip.IP4) *Lease {
	for _, l := range leases {
		if pubIP == l.Attrs.PublicIP {
			return &l
		}
	}

	return nil
}

func (m *LocalManager) tryAcquireLease(ctx context.Context, network string, config *Config, extIaddr ip.IP4, attrs *LeaseAttrs) (*Lease, error) {
	var err error
	leases, _, err := m.registry.getSubnets(ctx, network)
	if err != nil {
		return nil, err
	}

	// try to reuse a subnet if there's one that matches our IP
	if l := findLeaseByIP(leases, extIaddr); l != nil {
		// make sure the existing subnet is still within the configured network
		if isSubnetConfigCompat(config, l.Subnet) {
			log.Infof("Found lease (%v) for current IP (%v), reusing", l.Subnet, extIaddr)
			exp, err := m.registry.updateSubnet(ctx, network, l.Subnet, attrs, subnetTTL, 0)
			if err != nil {
				return nil, err
			}

			l.Attrs = *attrs
			l.Expiration = exp
			return l, nil
		} else {
			log.Infof("Found lease (%v) for current IP (%v) but not compatible with current config, deleting", l.Subnet, extIaddr)
			if err := m.registry.deleteSubnet(ctx, network, l.Subnet); err != nil {
				return nil, err
			}
		}
	}

	// no existing match, grab a new one
	sn, err := m.allocateSubnet(config, leases)
	if err != nil {
		return nil, err
	}

	exp, err := m.registry.createSubnet(ctx, network, sn, attrs, subnetTTL)
	if err == nil {
		return &Lease{
			Subnet:     sn,
			Attrs:      *attrs,
			Expiration: exp,
		}, nil
	}

	if etcdErr, ok := err.(etcd.Error); ok && etcdErr.Code == etcd.ErrorCodeNodeExist {
		// if etcd returned Key Already Exists, try again.
		return nil, nil
	}

	return nil, err
}

func (m *LocalManager) allocateSubnet(config *Config, leases []Lease) (ip.IP4Net, error) {
	log.Infof("Picking subnet in range %s ... %s", config.SubnetMin, config.SubnetMax)

	var bag []ip.IP4
	sn := ip.IP4Net{IP: config.SubnetMin, PrefixLen: config.SubnetLen}

OuterLoop:
	for ; sn.IP <= config.SubnetMax && len(bag) < 100; sn = sn.Next() {
		for _, l := range leases {
			if sn.Overlaps(l.Subnet) {
				continue OuterLoop
			}
		}
		bag = append(bag, sn.IP)
	}

	if len(bag) == 0 {
		return ip.IP4Net{}, errors.New("out of subnets")
	} else {
		i := randInt(0, len(bag))
		return ip.IP4Net{IP: bag[i], PrefixLen: config.SubnetLen}, nil
	}
}

func (m *LocalManager) RenewLease(ctx context.Context, network string, lease *Lease) error {
	// TODO(eyakubovich): propogate ctx into registry
	exp, err := m.registry.updateSubnet(ctx, network, lease.Subnet, &lease.Attrs, subnetTTL, 0)
	if err != nil {
		return err
	}

	lease.Expiration = exp
	return nil
}

func getNextIndex(cursor interface{}) (uint64, error) {
	nextIndex := uint64(0)

	if wc, ok := cursor.(watchCursor); ok {
		nextIndex = wc.index
	} else if s, ok := cursor.(string); ok {
		var err error
		nextIndex, err = strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse cursor: %v", err)
		}
	} else {
		return 0, fmt.Errorf("internal error: watch cursor is of unknown type")
	}

	return nextIndex, nil
}

func (m *LocalManager) WatchLeases(ctx context.Context, network string, cursor interface{}) (LeaseWatchResult, error) {
	if cursor == nil {
		return m.leaseWatchReset(ctx, network)
	}

	nextIndex, err := getNextIndex(cursor)
	if err != nil {
		return LeaseWatchResult{}, err
	}

	evt, index, err := m.registry.watchSubnets(ctx, network, nextIndex)

	switch {
	case err == nil:
		return LeaseWatchResult{
			Events: []Event{evt},
			Cursor: watchCursor{index},
		}, nil

	case isIndexTooSmall(err):
		log.Warning("Watch of subnet leases failed because etcd index outside history window")
		return m.leaseWatchReset(ctx, network)

	default:
		return LeaseWatchResult{}, err
	}
}

func (m *LocalManager) WatchNetworks(ctx context.Context, cursor interface{}) (NetworkWatchResult, error) {
	if cursor == nil {
		return m.networkWatchReset(ctx)
	}

	nextIndex, err := getNextIndex(cursor)
	if err != nil {
		return NetworkWatchResult{}, err
	}

DoWatch:
	evt, index, err := m.registry.watchNetworks(ctx, nextIndex)

	switch {
	case err == nil:
		return NetworkWatchResult{
			Events: []Event{evt},
			Cursor: watchCursor{index},
		}, nil

	case err == ErrTryAgain:
		nextIndex = index
		goto DoWatch

	case isIndexTooSmall(err):
		log.Warning("Watch of networks failed because etcd index outside history window")
		return m.networkWatchReset(ctx)

	default:
		return NetworkWatchResult{}, err
	}
}

func isIndexTooSmall(err error) bool {
	etcdErr, ok := err.(etcd.Error)
	return ok && etcdErr.Code == etcd.ErrorCodeEventIndexCleared
}

// leaseWatchReset is called when incremental lease watch failed and we need to grab a snapshot
func (m *LocalManager) leaseWatchReset(ctx context.Context, network string) (LeaseWatchResult, error) {
	wr := LeaseWatchResult{}

	leases, index, err := m.registry.getSubnets(ctx, network)
	if err != nil {
		return wr, fmt.Errorf("failed to retrieve subnet leases: %v", err)
	}

	wr.Cursor = watchCursor{index}
	wr.Snapshot = leases
	return wr, nil
}

// networkWatchReset is called when incremental network watch failed and we need to grab a snapshot
func (m *LocalManager) networkWatchReset(ctx context.Context) (NetworkWatchResult, error) {
	wr := NetworkWatchResult{}

	networks, index, err := m.registry.getNetworks(ctx)
	if err != nil {
		return wr, fmt.Errorf("failed to retrieve networks: %v", err)
	}

	wr.Cursor = watchCursor{index}
	wr.Snapshot = networks
	return wr, nil
}

func isSubnetConfigCompat(config *Config, sn ip.IP4Net) bool {
	if sn.IP < config.SubnetMin || sn.IP > config.SubnetMax {
		return false
	}

	return sn.PrefixLen == config.SubnetLen
}
