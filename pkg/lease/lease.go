// Copyright 2022 flannel authors
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

package lease

import (
	"encoding/json"
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	log "k8s.io/klog/v2"
)

const (
	EventAdded EventType = iota
	EventRemoved
)

type (
	EventType int

	Event struct {
		Type  EventType `json:"type"`
		Lease Lease     `json:"lease,omitempty"`
	}
)

// LeaseAttrs includes extra information for the lease
type LeaseAttrs struct {
	PublicIP      ip.IP4
	PublicIPv6    *ip.IP6
	BackendType   string          `json:",omitempty"`
	BackendData   json.RawMessage `json:",omitempty"`
	BackendV6Data json.RawMessage `json:",omitempty"`
}

// Lease includes information about the lease 
type Lease struct {
	EnableIPv4 bool
	EnableIPv6 bool
	Subnet     ip.IP4Net
	IPv6Subnet ip.IP6Net
	Attrs      LeaseAttrs
	Expiration time.Time

	Asof int64 //Only used in etcd
}

type LeaseWatchResult struct {
	// Either Events or Snapshot will be set.  If Events is empty, it means
	// the cursor was out of range and Snapshot contains the current list
	// of items, even if empty.
	Events   []Event     `json:"events"`
	Snapshot []Lease     `json:"snapshot"` //Only used in etcd
	Cursor   interface{} `json:"cursor"`   //Only used in etcd
}

type LeaseWatcher struct {
	OwnLease *Lease //Lease with the subnet of the local node
	Leases   []Lease //Leases with subnets from other nodes
}

// Reset is called by etcd-subnet when using a snapshot
func (lw *LeaseWatcher) Reset(leases []Lease) []Event {
	batch := []Event{}

	for _, nl := range leases {
		found := false
		if sameSubnet(nl.EnableIPv4, nl.EnableIPv6, *lw.OwnLease, nl) {
			continue
		}

		for i, ol := range lw.Leases {
			if sameSubnet(ol.EnableIPv4, ol.EnableIPv6, ol, nl) {
				lw.Leases = append(lw.Leases[:i], lw.Leases[i+1:]...)
				found = true
				break
			}
		}

		if !found {
			// new lease
			batch = append(batch, Event{EventAdded, nl})
		}
	}

	for _, l := range lw.Leases {
		batch = append(batch, Event{EventRemoved, l})
	}

	// copy the leases over (caution: don't just assign a slice)
	lw.Leases = make([]Lease, len(leases))
	copy(lw.Leases, leases)

	return batch
}

// Update reads the leases in the events and depending on Type, adds them or removes them
func (lw *LeaseWatcher) Update(events []Event) []Event {
	batch := []Event{}

	for _, e := range events {
		if sameSubnet(e.Lease.EnableIPv4, e.Lease.EnableIPv6, *lw.OwnLease, e.Lease) {
			continue
		}

		switch e.Type {
		case EventAdded:
			batch = append(batch, lw.add(&e.Lease))

		case EventRemoved:
			batch = append(batch, lw.remove(&e.Lease))
		}
	}

	return batch
}

// add updates lw.Leases, adding the passed lease (either overwriting or appending). It makes lw.Leases a set
func (lw *LeaseWatcher) add(lease *Lease) Event {
	for i, l := range lw.Leases {
		if sameSubnet(l.EnableIPv4, l.EnableIPv6, l, *lease) {
			lw.Leases[i] = *lease
			return Event{EventAdded, lw.Leases[i]}
		}
	}

	lw.Leases = append(lw.Leases, *lease)

	return Event{EventAdded, lw.Leases[len(lw.Leases)-1]}
}

// remove updates lw.Leases, removing the passed lease
func (lw *LeaseWatcher) remove(lease *Lease) Event {
	for i, l := range lw.Leases {
		if sameSubnet(l.EnableIPv4, l.EnableIPv6, l, *lease) {
			lw.Leases = append(lw.Leases[:i], lw.Leases[i+1:]...)
			return Event{EventRemoved, l}
		}
	}

	log.Errorf("Removed subnet (%s) and ipv6 subnet (%s) were not found", lease.Subnet, lease.IPv6Subnet)
	return Event{EventRemoved, *lease}
}

// sameSubnet checks if the subnets are the same in ipv4-only, ipv6-only and dualStack cases
func sameSubnet(ipv4Enabled, ipv6Enabled bool, firstLease, secondLease Lease) bool {
	// ipv4 only case
	if ipv4Enabled && !ipv6Enabled && firstLease.Subnet.Equal(secondLease.Subnet) {
		return true
	}
	// ipv6 only case
	if !ipv4Enabled && ipv6Enabled && firstLease.IPv6Subnet.Equal(secondLease.IPv6Subnet) {
		return true
	}
	// dualStack case
	if ipv4Enabled && ipv6Enabled && firstLease.Subnet.Equal(secondLease.Subnet) && firstLease.IPv6Subnet.Equal(secondLease.IPv6Subnet) {
		return true
	}
	// etcd case
	if !ipv4Enabled && !ipv6Enabled && firstLease.Subnet.Equal(secondLease.Subnet) {
		return true
	}

	return false
}
