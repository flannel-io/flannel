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
	"time"

	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
)

// WatchLeases performs a long term watch of the given network's subnet leases
// and communicates addition/deletion events on receiver channel. It takes care
// of handling "fall-behind" logic where the history window has advanced too far
// and it needs to diff the latest snapshot with its saved state and generate events
func WatchLeases(ctx context.Context, sm Manager, network string, receiver chan []Event) {
	lw := &leaseWatcher{}
	var cursor interface{}

	for {
		res, err := sm.WatchLeases(ctx, network, cursor)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				return
			}

			log.Errorf("Watch subnets: %v", err)
			time.Sleep(time.Second)
			continue
		}
		cursor = res.Cursor

		batch := []Event{}

		if len(res.Snapshot) > 0 {
			batch = lw.reset(res.Snapshot)
		} else {
			batch = lw.update(res.Events)
		}

		if batch != nil {
			receiver <- batch
		}
	}
}

type leaseWatcher struct {
	leases []Lease
}

func (lw *leaseWatcher) reset(leases []Lease) []Event {
	batch := []Event{}

	for _, nl := range leases {
		found := false
		for i, ol := range lw.leases {
			if ol.Subnet.Equal(nl.Subnet) {
				lw.leases = deleteLease(lw.leases, i)
				found = true
				break
			}
		}

		if !found {
			// new lease
			batch = append(batch, Event{SubnetAdded, nl})
		}
	}

	// everything left in sm.leases has been deleted
	for _, l := range lw.leases {
		batch = append(batch, Event{SubnetRemoved, l})
	}

	lw.leases = leases

	return batch
}

func (lw *leaseWatcher) update(events []Event) []Event {
	batch := []Event{}

	for _, e := range events {
		switch e.Type {
		case SubnetAdded:
			batch = append(batch, lw.add(&e.Lease))

		case SubnetRemoved:
			batch = append(batch, lw.remove(&e.Lease))
		}
	}

	return batch
}

func (lw *leaseWatcher) add(lease *Lease) Event {
	for i, l := range lw.leases {
		if l.Subnet.Equal(lease.Subnet) {
			lw.leases[i] = *lease
			return Event{SubnetAdded, lw.leases[i]}
		}
	}

	lw.leases = append(lw.leases, *lease)
	return Event{SubnetAdded, lw.leases[len(lw.leases)-1]}
}

func (lw *leaseWatcher) remove(lease *Lease) Event {
	for i, l := range lw.leases {
		if l.Subnet.Equal(lease.Subnet) {
			lw.leases = deleteLease(lw.leases, i)
			return Event{SubnetRemoved, l}
		}
	}

	log.Errorf("Removed subnet (%s) was not found", lease.Subnet)
	return Event{SubnetRemoved, *lease}
}

func deleteLease(l []Lease, i int) []Lease {
	l[i], l = l[len(l)-1], l[:len(l)-1]
	return l
}
