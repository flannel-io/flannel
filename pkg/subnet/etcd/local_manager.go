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
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/subnet"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"golang.org/x/net/context"
	log "k8s.io/klog"
)

const (
	raceRetries = 10
	subnetTTL   = 24 * time.Hour
)

var (
	errInterrupted   = errors.New("interrupted")
	errCanceled      = errors.New("canceled")
	errUnimplemented = errors.New("unimplemented")
)

type LocalManager struct {
	registry               Registry
	previousSubnet         ip.IP4Net
	previousIPv6Subnet     ip.IP6Net
	subnetLeaseRenewMargin int
}

type watchCursor struct {
	index int64
}

func isErrEtcdNodeExist(e error) bool {
	if e == nil {
		return false
	}
	return e == rpctypes.ErrDuplicateKey
}

func (c watchCursor) String() string {
	return strconv.FormatInt(c.index, 10)
}

func NewLocalManager(ctx context.Context, config *EtcdConfig, prevSubnet ip.IP4Net, prevIPv6Subnet ip.IP6Net, subnetLeaseRenewMargin int) (subnet.Manager, error) {
	r, err := newEtcdSubnetRegistry(ctx, config, nil)
	if err != nil {
		return nil, err
	}
	return newLocalManager(r, prevSubnet, prevIPv6Subnet, subnetLeaseRenewMargin), nil
}

func newLocalManager(r Registry, prevSubnet ip.IP4Net, prevIPv6Subnet ip.IP6Net, subnetLeaseRenewMargin int) subnet.Manager {
	return &LocalManager{
		registry:               r,
		previousSubnet:         prevSubnet,
		previousIPv6Subnet:     prevIPv6Subnet,
		subnetLeaseRenewMargin: subnetLeaseRenewMargin,
	}
}

func (m *LocalManager) GetNetworkConfig(ctx context.Context) (*subnet.Config, error) {
	cfg, err := m.registry.getNetworkConfig(ctx)
	if err != nil {
		return nil, err
	}

	config, err := subnet.ParseConfig(cfg)
	if err != nil {
		return nil, err
	}
	err = subnet.CheckNetworkConfig(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (m *LocalManager) AcquireLease(ctx context.Context, attrs *subnet.LeaseAttrs) (*subnet.Lease, error) {
	config, err := m.GetNetworkConfig(ctx)
	if err != nil {
		return nil, err
	}

	for i := 0; i < raceRetries; i++ {
		l, err := m.tryAcquireLease(ctx, config, attrs.PublicIP, attrs)
		switch err {
		case nil:
			return l, nil
		case errTryAgain:
			continue
		default:
			return nil, err
		}
	}

	return nil, errors.New("Max retries reached trying to acquire a subnet")
}

func findLeaseByIP(leases []subnet.Lease, pubIP ip.IP4) *subnet.Lease {
	for _, l := range leases {
		if pubIP == l.Attrs.PublicIP {
			return &l
		}
	}

	return nil
}

func findLeaseBySubnet(leases []subnet.Lease, subnet ip.IP4Net) *subnet.Lease {
	for _, l := range leases {
		if subnet.Equal(l.Subnet) {
			return &l
		}
	}

	return nil
}

func (m *LocalManager) tryAcquireLease(ctx context.Context, config *subnet.Config, extIaddr ip.IP4, attrs *subnet.LeaseAttrs) (*subnet.Lease, error) {
	leases, _, err := m.registry.getSubnets(ctx)
	if err != nil {
		return nil, err
	}

	// Try to reuse a subnet if there's one that matches our IP
	if l := findLeaseByIP(leases, extIaddr); l != nil {
		// Make sure the existing subnet is still within the configured network
		if isSubnetConfigCompat(config, l.Subnet) && isIPv6SubnetConfigCompat(config, l.IPv6Subnet) {
			log.Infof("Found lease (ip: %v ipv6: %v) for current IP (%v), reusing", l.Subnet, l.IPv6Subnet, extIaddr)

			ttl := time.Duration(0)
			if !l.Expiration.IsZero() {
				// Not a reservation
				ttl = subnetTTL
			}
			exp, err := m.registry.updateSubnet(ctx, l.Subnet, l.IPv6Subnet, attrs, ttl, 0)
			if err != nil {
				return nil, err
			}

			l.Attrs = *attrs
			l.Expiration = exp
			return l, nil
		} else {
			log.Infof("Found lease (%+v) for current IP (%v) but not compatible with current config, deleting", l, extIaddr)
			if err := m.registry.deleteSubnet(ctx, l.Subnet, l.IPv6Subnet); err != nil {
				return nil, err
			}
		}
	}

	// no existing match, check if there was a previous subnet to use
	var sn ip.IP4Net
	var sn6 ip.IP6Net
	if !m.previousSubnet.Empty() {
		// use previous subnet
		if l := findLeaseBySubnet(leases, m.previousSubnet); l == nil {
			// Check if the previous subnet is a part of the network and of the right subnet length
			if isSubnetConfigCompat(config, m.previousSubnet) && isIPv6SubnetConfigCompat(config, m.previousIPv6Subnet) {
				log.Infof("Found previously leased subnet (%v), reusing", m.previousSubnet)
				sn = m.previousSubnet
				sn6 = m.previousIPv6Subnet
			} else {
				log.Errorf("Found previously leased subnet (%v) that is not compatible with the Etcd network config, ignoring", m.previousSubnet)
			}
		}
	}

	if sn.Empty() {
		// no existing match, grab a new one
		sn, sn6, err = m.allocateSubnet(config, leases)
		if err != nil {
			return nil, err
		}
	}

	exp, err := m.registry.createSubnet(ctx, sn, sn6, attrs, subnetTTL)
	switch {
	case err == nil:
		log.Infof("Allocated lease (ip: %v ipv6: %v) to current node (%v) ", sn, sn6, extIaddr)
		return &subnet.Lease{
			EnableIPv4: true,
			Subnet:     sn,
			EnableIPv6: !sn6.Empty(),
			IPv6Subnet: sn6,
			Attrs:      *attrs,
			Expiration: exp,
		}, nil
	case isErrEtcdNodeExist(err):
		return nil, errTryAgain
	default:
		return nil, err
	}
}

func (m *LocalManager) allocateSubnet(config *subnet.Config, leases []subnet.Lease) (ip.IP4Net, ip.IP6Net, error) {
	log.Infof("Picking subnet in range %s ... %s", config.SubnetMin, config.SubnetMax)
	if config.EnableIPv6 {
		log.Infof("Picking ipv6 subnet in range %s ... %s", config.IPv6SubnetMin, config.IPv6SubnetMax)
	}

	var availableIPs []ip.IP4
	var availableIPv6s []*ip.IP6

	sn := ip.IP4Net{IP: config.SubnetMin, PrefixLen: config.SubnetLen}
	var sn6 ip.IP6Net
	if config.EnableIPv6 {
		sn6 = ip.IP6Net{IP: config.IPv6SubnetMin, PrefixLen: config.IPv6SubnetLen}
	}

OuterLoop:
	for ; sn.IP <= config.SubnetMax && len(availableIPs) < 100; sn = sn.Next() {
		for _, l := range leases {
			if sn.Overlaps(l.Subnet) {
				continue OuterLoop
			}
		}
		availableIPs = append(availableIPs, sn.IP)
	}

	if !sn6.Empty() {
	OuterLoopv6:
		for ; sn6.IP.Cmp(config.IPv6SubnetMax) <= 0 && len(availableIPv6s) < 100; sn6 = sn6.Next() {
			for _, l := range leases {
				if sn6.Overlaps(l.IPv6Subnet) {
					continue OuterLoopv6
				}
			}
			availableIPv6s = append(availableIPv6s, sn6.IP)
		}
	}

	if len(availableIPs) == 0 || (!sn6.Empty() && len(availableIPv6s) == 0) {
		return ip.IP4Net{}, ip.IP6Net{}, errors.New("out of subnets")
	} else {
		i := randInt(0, len(availableIPs))
		ipnet := ip.IP4Net{IP: availableIPs[i], PrefixLen: config.SubnetLen}

		if sn6.Empty() {
			return ipnet, ip.IP6Net{}, nil
		}
		i = randInt(0, len(availableIPv6s))
		return ipnet, ip.IP6Net{IP: availableIPv6s[i], PrefixLen: config.IPv6SubnetLen}, nil
	}
}

func (m *LocalManager) RenewLease(ctx context.Context, lease *subnet.Lease) error {
	exp, err := m.registry.updateSubnet(ctx, lease.Subnet, lease.IPv6Subnet, &lease.Attrs, subnetTTL, 0)
	if err != nil {
		return err
	}

	lease.Expiration = exp
	return nil
}

func getNextIndex(cursor interface{}) (int64, error) {
	nextIndex := int64(0)

	if wc, ok := cursor.(watchCursor); ok {
		nextIndex = wc.index
	} else if s, ok := cursor.(string); ok {
		var err error
		nextIndex, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse cursor: %v", err)
		}
	} else {
		return 0, fmt.Errorf("internal error: watch cursor is of unknown type")
	}

	return nextIndex + 1, nil
}

func (m *LocalManager) leaseWatchReset(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) (subnet.LeaseWatchResult, error) {
	l, index, err := m.registry.getSubnet(ctx, sn, sn6)
	if err != nil {
		return subnet.LeaseWatchResult{}, err
	}

	return subnet.LeaseWatchResult{
		Snapshot: []subnet.Lease{*l},
		Cursor:   watchCursor{index},
	}, nil
}

func (m *LocalManager) WatchLease(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, receiver chan []subnet.LeaseWatchResult) error {
	wr, err := m.leaseWatchReset(ctx, sn, sn6)
	if err != nil {
		return err
	}

	log.Info("manager.WatchLease: sending reset results...")
	//send the result of leaseWatchResult to allow the listener
	//to catch-up to the current state
	receiver <- []subnet.LeaseWatchResult{wr}

	nextIndex, err := getNextIndex(wr.Cursor)
	if err != nil {
		return err
	}

	err = m.registry.watchSubnet(ctx, nextIndex, sn, sn6, receiver)
	if err != nil {
		return err
	}
	return nil
}

func (m *LocalManager) WatchLeases(ctx context.Context, receiver chan []subnet.LeaseWatchResult) error {
	wr, err := m.registry.leasesWatchReset(ctx)
	if err != nil {
		return err
	}

	// send the result of leasesWatchReset to the listener
	// to catch-up on the state if the registry
	// before starting to watch changes
	receiver <- []subnet.LeaseWatchResult{wr}

	nextIndex, err := getNextIndex(wr.Cursor)
	if err != nil {
		return err
	}

	err = m.registry.watchSubnets(ctx, receiver, nextIndex)
	if err != nil {
		return err
	}
	return nil
}

// CompleteLease monitor lease
func (m *LocalManager) CompleteLease(ctx context.Context, lease *subnet.Lease, wg *sync.WaitGroup) error {
	// Use the subnet manager to start watching leases.
	evts := make(chan subnet.Event)

	wg.Add(1)
	go func() {
		l := lease
		subnet.WatchLease(ctx, m, l.Subnet, l.IPv6Subnet, evts)
		wg.Done()
	}()

	renewMargin := time.Duration(m.subnetLeaseRenewMargin) * time.Minute
	dur := time.Until(lease.Expiration) - renewMargin

	for {
		select {
		case <-time.After(dur):
			err := m.RenewLease(ctx, lease)
			if err != nil {
				log.Error("Error renewing lease (trying again in 1 min): ", err)
				dur = time.Minute
				continue
			}

			log.Info("Lease renewed, new expiration: ", lease.Expiration)
			dur = time.Until(lease.Expiration) - renewMargin

		case e, ok := <-evts:
			if !ok {
				log.Infof("Stopped monitoring lease")
				return errCanceled
			}
			switch e.Type {
			case subnet.EventAdded:
				lease.Expiration = e.Lease.Expiration
				dur = time.Until(lease.Expiration) - renewMargin
				log.Infof("Waiting for %s to renew lease", dur)

			case subnet.EventRemoved:
				log.Error("Lease has been revoked. Shutting down daemon.")
				return errInterrupted
			}
		}
	}
}

func isIndexTooSmall(err error) bool {
	return err == rpctypes.ErrGRPCCompacted
}

func isSubnetConfigCompat(config *subnet.Config, sn ip.IP4Net) bool {
	if sn.IP < config.SubnetMin || sn.IP > config.SubnetMax {
		return false
	}

	return sn.PrefixLen == config.SubnetLen
}

func isIPv6SubnetConfigCompat(config *subnet.Config, sn6 ip.IP6Net) bool {
	if !config.EnableIPv6 {
		return sn6.Empty()
	}
	if sn6.Empty() || sn6.IP.Cmp(config.IPv6SubnetMin) < 0 || sn6.IP.Cmp(config.IPv6SubnetMax) > 0 {
		return false
	}

	return sn6.PrefixLen == config.IPv6SubnetLen
}

func (m *LocalManager) Name() string {
	previousSubnet := m.previousSubnet.String()
	if m.previousSubnet.Empty() {
		previousSubnet = "None"
	}
	return fmt.Sprintf("Etcd Local Manager with Previous Subnet: %s", previousSubnet)
}

// For etcd subnet manager, the file never changes so we just write it once at startup
func (m *LocalManager) HandleSubnetFile(path string, config *subnet.Config, ipMasq bool, sn ip.IP4Net, ipv6sn ip.IP6Net, mtu int) error {
	return subnet.WriteSubnetFile(path, config, ipMasq, sn, ipv6sn, mtu)
}
