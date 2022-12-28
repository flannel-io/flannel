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
	log "k8s.io/klog"
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

// LeaseAttrs (is it used by both Kube and etcd?)
type LeaseAttrs struct {
	PublicIP      ip.IP4
	PublicIPv6    *ip.IP6
	BackendType   string          `json:",omitempty"`
	BackendData   json.RawMessage `json:",omitempty"`
	BackendV6Data json.RawMessage `json:",omitempty"`
}

// Lease (is it used by both Kube and etcd?)
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
	OwnLease *Lease
	Leases   []Lease
}

func (lw *LeaseWatcher) Reset(leases []Lease) []Event {
	batch := []Event{}

	for _, nl := range leases {
		if lw.OwnLease != nil && nl.EnableIPv4 && !nl.EnableIPv6 &&
			nl.Subnet.Equal(lw.OwnLease.Subnet) {
			continue
		} else if lw.OwnLease != nil && !nl.EnableIPv4 && nl.EnableIPv6 &&
			nl.IPv6Subnet.Equal(lw.OwnLease.IPv6Subnet) {
			continue
		} else if lw.OwnLease != nil && nl.EnableIPv4 && nl.EnableIPv6 &&
			nl.Subnet.Equal(lw.OwnLease.Subnet) &&
			nl.IPv6Subnet.Equal(lw.OwnLease.IPv6Subnet) {
			continue
		} else if lw.OwnLease != nil && !nl.EnableIPv4 && !nl.EnableIPv6 &&
			nl.Subnet.Equal(lw.OwnLease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
			continue
		}

		found := false
		for i, ol := range lw.Leases {
			if ol.EnableIPv4 && !ol.EnableIPv6 && ol.Subnet.Equal(nl.Subnet) {
				lw.Leases = deleteLease(lw.Leases, i)
				found = true
				break
			} else if ol.EnableIPv4 && !ol.EnableIPv6 && ol.IPv6Subnet.Equal(nl.IPv6Subnet) {
				lw.Leases = deleteLease(lw.Leases, i)
				found = true
				break
			} else if ol.EnableIPv4 && ol.EnableIPv6 && ol.Subnet.Equal(nl.Subnet) &&
				ol.IPv6Subnet.Equal(nl.IPv6Subnet) {
				lw.Leases = deleteLease(lw.Leases, i)
				found = true
				break
			} else if !ol.EnableIPv4 && !ol.EnableIPv6 && ol.Subnet.Equal(nl.Subnet) {
				//TODO - dual-stack temporarily only compatible with kube subnet manager
				lw.Leases = deleteLease(lw.Leases, i)
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
		if lw.OwnLease != nil && l.EnableIPv4 && !l.EnableIPv6 &&
			l.Subnet.Equal(lw.OwnLease.Subnet) {
			continue
		} else if lw.OwnLease != nil && !l.EnableIPv4 && l.EnableIPv6 &&
			l.IPv6Subnet.Equal(lw.OwnLease.IPv6Subnet) {
			continue
		} else if lw.OwnLease != nil && l.EnableIPv4 && l.EnableIPv6 &&
			l.Subnet.Equal(lw.OwnLease.Subnet) &&
			l.IPv6Subnet.Equal(lw.OwnLease.IPv6Subnet) {
			continue
		} else if lw.OwnLease != nil && !l.EnableIPv4 && !l.EnableIPv6 &&
			l.Subnet.Equal(lw.OwnLease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
			continue
		}
		batch = append(batch, Event{EventRemoved, l})
	}

	// copy the leases over (caution: don't just assign a slice)
	lw.Leases = make([]Lease, len(leases))
	copy(lw.Leases, leases)

	return batch
}

func (lw *LeaseWatcher) Update(events []Event) []Event {
	batch := []Event{}

	for _, e := range events {
		if lw.OwnLease != nil && e.Lease.EnableIPv4 && !e.Lease.EnableIPv6 &&
			e.Lease.Subnet.Equal(lw.OwnLease.Subnet) {
			continue
		} else if lw.OwnLease != nil && !e.Lease.EnableIPv4 && e.Lease.EnableIPv6 &&
			e.Lease.IPv6Subnet.Equal(lw.OwnLease.IPv6Subnet) {
			continue
		} else if lw.OwnLease != nil && e.Lease.EnableIPv4 && e.Lease.EnableIPv6 &&
			e.Lease.Subnet.Equal(lw.OwnLease.Subnet) &&
			e.Lease.IPv6Subnet.Equal(lw.OwnLease.IPv6Subnet) {
			continue
		} else if lw.OwnLease != nil && !e.Lease.EnableIPv4 && !e.Lease.EnableIPv6 &&
			e.Lease.Subnet.Equal(lw.OwnLease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
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

func (lw *LeaseWatcher) add(lease *Lease) Event {
	for i, l := range lw.Leases {
		if l.EnableIPv4 && !l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) {
			lw.Leases[i] = *lease
			return Event{EventAdded, lw.Leases[i]}
		} else if !l.EnableIPv4 && l.EnableIPv6 && l.IPv6Subnet.Equal(lease.IPv6Subnet) {
			lw.Leases[i] = *lease
			return Event{EventAdded, lw.Leases[i]}
		} else if l.EnableIPv4 && l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) &&
			l.IPv6Subnet.Equal(lease.IPv6Subnet) {
			lw.Leases[i] = *lease
			return Event{EventAdded, lw.Leases[i]}
		} else if !l.EnableIPv4 && !l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
			lw.Leases[i] = *lease
			return Event{EventAdded, lw.Leases[i]}
		}
	}
	lw.Leases = append(lw.Leases, *lease)

	return Event{EventAdded, lw.Leases[len(lw.Leases)-1]}
}

func (lw *LeaseWatcher) remove(lease *Lease) Event {
	for i, l := range lw.Leases {
		if l.EnableIPv4 && !l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) {
			lw.Leases = deleteLease(lw.Leases, i)
			return Event{EventRemoved, l}
		} else if !l.EnableIPv4 && l.EnableIPv6 && l.IPv6Subnet.Equal(lease.IPv6Subnet) {
			lw.Leases = deleteLease(lw.Leases, i)
			return Event{EventRemoved, l}
		} else if l.EnableIPv4 && l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) &&
			l.IPv6Subnet.Equal(lease.IPv6Subnet) {
			lw.Leases = deleteLease(lw.Leases, i)
			return Event{EventRemoved, l}
		} else if !l.EnableIPv4 && !l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
			lw.Leases = deleteLease(lw.Leases, i)
			return Event{EventRemoved, l}
		}
	}

	log.Errorf("Removed subnet (%s) and ipv6 subnet (%s) were not found", lease.Subnet, lease.IPv6Subnet)
	return Event{EventRemoved, *lease}
}

func deleteLease(l []Lease, i int) []Lease {
	l = append(l[:i], l[i+1:]...)
	return l
}