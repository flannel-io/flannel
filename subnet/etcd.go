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

	etcd "github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/etcd/client"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/pkg/ip"
)

const (
	registerRetries = 10
	subnetTTL       = 24 * time.Hour
)

type EtcdManager struct {
	registry Registry
}

var (
	subnetRegex *regexp.Regexp = regexp.MustCompile(`(\d+\.\d+.\d+.\d+)-(\d+)`)
)

type watchCursor struct {
	index uint64
}

func (c watchCursor) String() string {
	return strconv.FormatUint(c.index, 10)
}

func NewEtcdManager(config *EtcdConfig) (Manager, error) {
	r, err := newEtcdSubnetRegistry(config)
	if err != nil {
		return nil, err
	}
	return &EtcdManager{r}, nil
}

func newEtcdManager(r Registry) Manager {
	return &EtcdManager{r}
}

func (m *EtcdManager) GetNetworkConfig(ctx context.Context, network string) (*Config, error) {
	cfgResp, err := m.registry.getConfig(ctx, network)
	if err != nil {
		return nil, err
	}

	return ParseConfig(cfgResp.Node.Value)
}

func (m *EtcdManager) AcquireLease(ctx context.Context, network string, attrs *LeaseAttrs) (*Lease, error) {
	config, err := m.GetNetworkConfig(ctx, network)
	if err != nil {
		return nil, err
	}

	for {
		l, err := m.acquireLeaseOnce(ctx, network, config, attrs)
		switch {
		case err == nil:
			log.Info("Subnet lease acquired: ", l.Subnet)
			return l, nil

		case err == context.Canceled, err == context.DeadlineExceeded:
			return nil, err

		default:
			log.Error("Failed to acquire subnet: ", err)
		}

		select {
		case <-time.After(time.Second):

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func findLeaseByIP(leases []Lease, pubIP ip.IP4) *Lease {
	for _, l := range leases {
		if pubIP == l.Attrs.PublicIP {
			return &l
		}
	}

	return nil
}

func (m *EtcdManager) tryAcquireLease(ctx context.Context, network string, config *Config, extIaddr ip.IP4, attrs *LeaseAttrs) (*Lease, error) {
	var err error
	leases, _, err := m.getLeases(ctx, network)
	if err != nil {
		return nil, err
	}

	attrBytes, err := json.Marshal(attrs)
	if err != nil {
		return nil, err
	}

	// try to reuse a subnet if there's one that matches our IP
	if l := findLeaseByIP(leases, extIaddr); l != nil {
		// make sure the existing subnet is still within the configured network
		if isSubnetConfigCompat(config, l.Subnet) {
			log.Infof("Found lease (%v) for current IP (%v), reusing", l.Subnet, extIaddr)
			resp, err := m.registry.updateSubnet(ctx, network, l.Key(), string(attrBytes), subnetTTL)
			if err != nil {
				return nil, err
			}

			l.Attrs = attrs
			l.Expiration = *resp.Node.Expiration
			return l, nil
		} else {
			log.Infof("Found lease (%v) for current IP (%v) but not compatible with current config, deleting", l.Subnet, extIaddr)
			if _, err := m.registry.deleteSubnet(ctx, network, l.Key()); err != nil {
				return nil, err
			}
		}
	}

	// no existing match, grab a new one
	sn, err := m.allocateSubnet(config, leases)
	if err != nil {
		return nil, err
	}

	resp, err := m.registry.createSubnet(ctx, network, sn.StringSep(".", "-"), string(attrBytes), subnetTTL)
	if err == nil {
		return &Lease{
			Subnet:     sn,
			Attrs:      attrs,
			Expiration: *resp.Node.Expiration,
		}, nil
	}

	if etcdErr, ok := err.(etcd.Error); ok && etcdErr.Code == etcd.ErrorCodeNodeExist {
		// if etcd returned Key Already Exists, try again.
		return nil, nil
	}

	return nil, err
}

func (m *EtcdManager) acquireLeaseOnce(ctx context.Context, network string, config *Config, attrs *LeaseAttrs) (*Lease, error) {
	for i := 0; i < registerRetries; i++ {
		l, err := m.tryAcquireLease(ctx, network, config, attrs.PublicIP, attrs)
		switch {
		case err != nil:
			return nil, err
		case l != nil:
			return l, nil
		}

		// before moving on, check for cancel
		// TODO(eyakubovich): propogate ctx deeper into registry
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	return nil, errors.New("Max retries reached trying to acquire a subnet")
}

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

func (m *EtcdManager) allocateSubnet(config *Config, leases []Lease) (ip.IP4Net, error) {
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

// getLeases queries etcd to get a list of currently allocated leases for a given network.
// It returns the leases along with the "as-of" etcd-index that can be used as the starting
// point for etcd watch.
func (m *EtcdManager) getLeases(ctx context.Context, network string) ([]Lease, uint64, error) {
	resp, err := m.registry.getSubnets(ctx, network)

	leases := []Lease{}

	if err == nil {
		for _, node := range resp.Node.Nodes {
			sn, err := parseSubnetKey(node.Key)
			if err == nil {
				attrs := &LeaseAttrs{}
				if err = json.Unmarshal([]byte(node.Value), attrs); err == nil {
					exp := time.Time{}
					if node.Expiration != nil {
						exp = *node.Expiration
					}

					lease := Lease{
						Subnet:     sn,
						Attrs:      attrs,
						Expiration: exp,
					}
					leases = append(leases, lease)
				}
			}
		}

		return leases, resp.Index, nil
	}

	if etcdErr, ok := err.(etcd.Error); ok && etcdErr.Code == etcd.ErrorCodeKeyNotFound {
		// key not found: treat it as empty set
		return leases, etcdErr.Index, nil
	}

	return nil, 0, err
}

func (m *EtcdManager) RenewLease(ctx context.Context, network string, lease *Lease) error {
	attrBytes, err := json.Marshal(lease.Attrs)
	if err != nil {
		return err
	}

	// TODO(eyakubovich): propogate ctx into registry
	resp, err := m.registry.updateSubnet(ctx, network, lease.Key(), string(attrBytes), subnetTTL)
	if err != nil {
		return err
	}

	lease.Expiration = *resp.Node.Expiration
	return nil
}

func (m *EtcdManager) WatchLeases(ctx context.Context, network string, cursor interface{}) (WatchResult, error) {
	if cursor == nil {
		return m.watchReset(ctx, network)
	}

	nextIndex := uint64(0)

	if wc, ok := cursor.(watchCursor); ok {
		nextIndex = wc.index
	} else if s, ok := cursor.(string); ok {
		var err error
		nextIndex, err = strconv.ParseUint(s, 10, 64)
		if err != nil {
			return WatchResult{}, fmt.Errorf("failed to parse cursor: %v", err)
		}
	} else {
		return WatchResult{}, fmt.Errorf("internal error: watch cursor is of unknown type")
	}

	resp, err := m.registry.watchSubnets(ctx, network, nextIndex)

	switch {
	case err == nil:
		return parseSubnetWatchResponse(resp)

	case isIndexTooSmall(err):
		log.Warning("Watch of subnet leases failed because etcd index outside history window")
		return m.watchReset(ctx, network)

	default:
		return WatchResult{}, err
	}
}

func isIndexTooSmall(err error) bool {
	etcdErr, ok := err.(etcd.Error)
	return ok && etcdErr.Code == etcd.ErrorCodeEventIndexCleared
}

func parseSubnetWatchResponse(resp *etcd.Response) (WatchResult, error) {
	sn, err := parseSubnetKey(resp.Node.Key)
	if err != nil {
		return WatchResult{}, fmt.Errorf("error parsing subnet IP: %s", resp.Node.Key)
	}

	evt := Event{}

	switch resp.Action {
	case "delete", "expire":
		evt = Event{
			SubnetRemoved,
			Lease{Subnet: sn},
		}

	default:
		attrs := &LeaseAttrs{}
		err := json.Unmarshal([]byte(resp.Node.Value), attrs)
		if err != nil {
			return WatchResult{}, err
		}

		exp := time.Time{}
		if resp.Node.Expiration != nil {
			exp = *resp.Node.Expiration
		}

		evt = Event{
			SubnetAdded,
			Lease{
				Subnet:     sn,
				Attrs:      attrs,
				Expiration: exp,
			},
		}
	}

	cursor := watchCursor{resp.Node.ModifiedIndex}

	return WatchResult{
		Cursor: cursor,
		Events: []Event{evt},
	}, nil
}

// watchReset is called when incremental watch failed and we need to grab a snapshot
func (m *EtcdManager) watchReset(ctx context.Context, network string) (WatchResult, error) {
	wr := WatchResult{}

	leases, index, err := m.getLeases(ctx, network)

	if err != nil {
		return wr, fmt.Errorf("failed to retrieve subnet leases: %v", err)
	}

	cursor := watchCursor{index}
	wr.Snapshot = leases
	wr.Cursor = cursor
	return wr, nil
}

func isSubnetConfigCompat(config *Config, sn ip.IP4Net) bool {
	if sn.IP < config.SubnetMin || sn.IP > config.SubnetMax {
		return false
	}

	return sn.PrefixLen == config.SubnetLen
}
