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

package subnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"time"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/pkg/task"
)

const (
	registerRetries = 10
	subnetTTL       = 24 * 3600
	renewMargin     = time.Hour
)

// etcd error codes
const (
	etcdKeyNotFound       = 100
	etcdKeyAlreadyExists  = 105
	etcdEventIndexCleared = 401
)

const (
	SubnetAdded = iota
	SubnetRemoved
)

var (
	subnetRegex *regexp.Regexp = regexp.MustCompile(`(\d+\.\d+.\d+.\d+)-(\d+)`)
)

type LeaseAttrs struct {
	PublicIP    ip.IP4
	BackendType string          `json:",omitempty"`
	BackendData json.RawMessage `json:",omitempty"`
}

type SubnetLease struct {
	Network ip.IP4Net
	Attrs   LeaseAttrs
}

type SubnetManager struct {
	registry  subnetRegistry
	config    *Config
	myLease   SubnetLease
	leaseExp  time.Time
	lastIndex uint64
	leases    []SubnetLease
}

type EventType int

type Event struct {
	Type  EventType
	Lease SubnetLease
}

type EventBatch []Event

func NewSubnetManager(config *EtcdConfig) (*SubnetManager, error) {
	esr, err := newEtcdSubnetRegistry(config)
	if err != nil {
		return nil, err
	}
	return newSubnetManager(esr)
}

func (sm *SubnetManager) AcquireLease(attrs *LeaseAttrs, cancel chan bool) (ip.IP4Net, error) {
	for {
		sn, err := sm.acquireLeaseOnce(attrs, cancel)
		switch {
		case err == nil:
			log.Info("Subnet lease acquired: ", sn)
			return sn, nil

		case err == task.ErrCanceled:
			return ip.IP4Net{}, err

		default:
			log.Error("Failed to acquire subnet: ", err)
		}

		select {
		case <-time.After(time.Second):

		case <-cancel:
			return ip.IP4Net{}, task.ErrCanceled
		}
	}
}

func findLeaseByIP(leases []SubnetLease, pubIP ip.IP4) *SubnetLease {
	for _, l := range leases {
		if pubIP == l.Attrs.PublicIP {
			return &l
		}
	}

	return nil
}

func (sm *SubnetManager) tryAcquireLease(extIP ip.IP4, attrs *LeaseAttrs) (ip.IP4Net, error) {
	var err error
	sm.leases, err = sm.getLeases()
	if err != nil {
		return ip.IP4Net{}, err
	}

	attrBytes, err := json.Marshal(attrs)
	if err != nil {
		log.Errorf("marshal failed: %#v, %v", attrs, err)
		return ip.IP4Net{}, err
	}

	// try to reuse a subnet if there's one that matches our IP
	if l := findLeaseByIP(sm.leases, extIP); l != nil {
		resp, err := sm.registry.updateSubnet(l.Network.StringSep(".", "-"), string(attrBytes), subnetTTL)
		if err != nil {
			return ip.IP4Net{}, err
		}

		sm.myLease.Network = l.Network
		sm.myLease.Attrs = *attrs
		sm.leaseExp = *resp.Node.Expiration
		return l.Network, nil
	}

	// no existing match, grab a new one
	sn, err := sm.allocateSubnet()
	if err != nil {
		return ip.IP4Net{}, err
	}

	resp, err := sm.registry.createSubnet(sn.StringSep(".", "-"), string(attrBytes), subnetTTL)
	switch {
	case err == nil:
		sm.myLease.Network = sn
		sm.myLease.Attrs = *attrs
		sm.leaseExp = *resp.Node.Expiration
		return sn, nil

	// if etcd returned Key Already Exists, try again.
	case err.(*etcd.EtcdError).ErrorCode == etcdKeyAlreadyExists:
		return ip.IP4Net{}, nil

	default:
		return ip.IP4Net{}, err
	}
}

func (sm *SubnetManager) acquireLeaseOnce(attrs *LeaseAttrs, cancel chan bool) (ip.IP4Net, error) {
	for i := 0; i < registerRetries; i++ {
		sn, err := sm.tryAcquireLease(attrs.PublicIP, attrs)
		switch {
		case err != nil:
			return ip.IP4Net{}, err
		case sn.IP != 0:
			return sn, nil
		}

		// before moving on, check for cancel
		if interrupted(cancel) {
			return ip.IP4Net{}, task.ErrCanceled
		}
	}

	return ip.IP4Net{}, errors.New("Max retries reached trying to acquire a subnet")
}

func (sm *SubnetManager) GetConfig() *Config {
	return sm.config
}

/// Implementation
func parseSubnetKey(s string) (ip.IP4Net, error) {
	if parts := subnetRegex.FindStringSubmatch(s); len(parts) == 3 {
		snIp := net.ParseIP(parts[1]).To4()
		prefixLen, err := strconv.ParseUint(parts[2], 10, 5)
		if snIp != nil && err == nil {
			return ip.IP4Net{IP: ip.FromIP(snIp), PrefixLen: uint(prefixLen)}, nil
		}
	}

	return ip.IP4Net{}, errors.New("Error parsing IP Subnet")
}

func newSubnetManager(r subnetRegistry) (*SubnetManager, error) {
	cfgResp, err := r.getConfig()
	if err != nil {
		return nil, err
	}

	cfg, err := ParseConfig(cfgResp.Node.Value)
	if err != nil {
		return nil, err
	}

	sm := SubnetManager{
		registry: r,
		config:   cfg,
	}

	return &sm, nil
}

func (sm *SubnetManager) getLeases() ([]SubnetLease, error) {
	resp, err := sm.registry.getSubnets()

	var leases []SubnetLease
	switch {
	case err == nil:
		for _, node := range resp.Node.Nodes {
			sn, err := parseSubnetKey(node.Key)
			if err == nil {
				var attrs LeaseAttrs
				if err = json.Unmarshal([]byte(node.Value), &attrs); err == nil {
					lease := SubnetLease{sn, attrs}
					leases = append(leases, lease)
				}
			}
		}
		sm.lastIndex = resp.EtcdIndex

	case err.(*etcd.EtcdError).ErrorCode == etcdKeyNotFound:
		// key not found: treat it as empty set
		sm.lastIndex = err.(*etcd.EtcdError).Index

	default:
		return nil, err
	}

	return leases, nil
}

func deleteLease(l []SubnetLease, i int) []SubnetLease {
	l[i], l = l[len(l)-1], l[:len(l)-1]
	return l
}

func (sm *SubnetManager) applyLeases(newLeases []SubnetLease) EventBatch {
	var batch EventBatch

	for _, l := range newLeases {
		// skip self
		if l.Network.Equal(sm.myLease.Network) {
			continue
		}

		found := false
		for i, c := range sm.leases {
			if c.Network.Equal(l.Network) {
				sm.leases = deleteLease(sm.leases, i)
				found = true
				break
			}
		}

		if !found {
			// new subnet
			batch = append(batch, Event{SubnetAdded, l})
		}
	}

	// everything left in sm.leases has been deleted
	for _, c := range sm.leases {
		batch = append(batch, Event{SubnetRemoved, c})
	}

	sm.leases = newLeases

	return batch
}

func (sm *SubnetManager) applySubnetChange(action string, ipn ip.IP4Net, data string) (Event, error) {
	switch action {
	case "delete", "expire":
		for i, l := range sm.leases {
			if l.Network.Equal(ipn) {
				deleteLease(sm.leases, i)
				return Event{SubnetRemoved, l}, nil
			}
		}

		log.Errorf("Removed subnet (%s) was not found", ipn)
		return Event{
			SubnetRemoved,
			SubnetLease{ipn, LeaseAttrs{}},
		}, nil

	default:
		var attrs LeaseAttrs
		err := json.Unmarshal([]byte(data), &attrs)
		if err != nil {
			return Event{}, err
		}

		for i, l := range sm.leases {
			if l.Network.Equal(ipn) {
				sm.leases[i] = SubnetLease{ipn, attrs}
				return Event{SubnetAdded, sm.leases[i]}, nil
			}
		}

		sm.leases = append(sm.leases, SubnetLease{ipn, attrs})
		return Event{SubnetAdded, sm.leases[len(sm.leases)-1]}, nil
	}
}

func (sm *SubnetManager) allocateSubnet() (ip.IP4Net, error) {
	log.Infof("Picking subnet in range %s ... %s", sm.config.SubnetMin, sm.config.SubnetMax)

	var bag []ip.IP4
	sn := ip.IP4Net{IP: sm.config.SubnetMin, PrefixLen: sm.config.SubnetLen}

OuterLoop:
	for ; sn.IP <= sm.config.SubnetMax && len(bag) < 100; sn = sn.Next() {
		for _, l := range sm.leases {
			if sn.Overlaps(l.Network) {
				continue OuterLoop
			}
		}
		bag = append(bag, sn.IP)
	}

	if len(bag) == 0 {
		return ip.IP4Net{}, errors.New("out of subnets")
	} else {
		i := randInt(0, len(bag))
		return ip.IP4Net{IP: bag[i], PrefixLen: sm.config.SubnetLen}, nil
	}
}

func (sm *SubnetManager) WatchLeases(receiver chan EventBatch, cancel chan bool) {
	// "catch up" by replaying all the leases we discovered during
	// AcquireLease
	var batch EventBatch
	for _, l := range sm.leases {
		if !sm.myLease.Network.Equal(l.Network) {
			batch = append(batch, Event{SubnetAdded, l})
		}
	}
	if len(batch) > 0 {
		receiver <- batch
	}

	for {
		resp, err := sm.registry.watchSubnets(sm.lastIndex+1, cancel)

		// watchSubnets exited by cancel chan being signaled
		if err == nil && resp == nil {
			return
		}

		var batch *EventBatch
		if err == nil {
			batch, err = sm.parseSubnetWatchResponse(resp)
		} else {
			batch, err = sm.parseSubnetWatchError(err)
		}

		if err != nil {
			log.Errorf("%v", err)
			time.Sleep(time.Second)
			continue
		}

		if batch != nil {
			receiver <- *batch
		}
	}
}

func (sm *SubnetManager) parseSubnetWatchResponse(resp *etcd.Response) (batch *EventBatch, err error) {
	sm.lastIndex = resp.Node.ModifiedIndex

	sn, err := parseSubnetKey(resp.Node.Key)
	if err != nil {
		err = fmt.Errorf("Error parsing subnet IP: %s", resp.Node.Key)
		return
	}

	// Don't process our own changes
	if !sm.myLease.Network.Equal(sn) {
		evt, err := sm.applySubnetChange(resp.Action, sn, resp.Node.Value)
		if err != nil {
			return nil, err
		}
		batch = &EventBatch{evt}
	}

	return
}

func (sm *SubnetManager) parseSubnetWatchError(err error) (batch *EventBatch, out error) {
	etcdErr, ok := err.(*etcd.EtcdError)
	if ok && etcdErr.ErrorCode == etcdEventIndexCleared {
		// etcd maintains a history window for events and it's possible to fall behind.
		// to recover, get the current state and then "diff" against our cache to generate
		// events for the caller
		log.Warning("Watch of subnet leases failed because etcd index outside history window")

		leases, err := sm.getLeases()
		if err == nil {
			lb := sm.applyLeases(leases)
			batch = &lb
		} else {
			out = fmt.Errorf("Failed to retrieve subnet leases: %v", err)
		}
	} else {
		out = fmt.Errorf("Watch of subnet leases failed: %v", err)
	}

	return
}

func (sm *SubnetManager) LeaseRenewer(cancel chan bool) {
	for {
		dur := sm.leaseExp.Sub(time.Now()) - renewMargin

		select {
		case <-time.After(dur):
			attrBytes, err := json.Marshal(&sm.myLease.Attrs)
			if err != nil {
				log.Error("Error renewing lease (trying again in 1 min): ", err)
				dur = time.Minute
				continue
			}

			resp, err := sm.registry.updateSubnet(sm.myLease.Network.StringSep(".", "-"), string(attrBytes), subnetTTL)
			if err != nil {
				log.Error("Error renewing lease (trying again in 1 min): ", err)
				dur = time.Minute
				continue
			}

			sm.leaseExp = *resp.Node.Expiration
			log.Info("Lease renewed, new expiration: ", sm.leaseExp)
			dur = sm.leaseExp.Sub(time.Now()) - renewMargin

		case <-cancel:
			return
		}
	}
}

func interrupted(cancel chan bool) bool {
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}
