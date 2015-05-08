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
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/flannel/pkg/ip"
)

type mockSubnetRegistry struct {
	subnets *etcd.Node
	addCh   chan string
	delCh   chan string
	index   uint64
	ttl     uint64
}

func newMockSubnetRegistry(ttlOverride uint64) *mockSubnetRegistry {
	subnodes := []*etcd.Node{
		&etcd.Node{Key: "10.3.1.0-24", Value: `{ "PublicIP": "1.1.1.1" }`, ModifiedIndex: 10},
		&etcd.Node{Key: "10.3.2.0-24", Value: `{ "PublicIP": "1.1.1.1" }`, ModifiedIndex: 11},
		&etcd.Node{Key: "10.3.4.0-24", Value: `{ "PublicIP": "1.1.1.1" }`, ModifiedIndex: 12},
		&etcd.Node{Key: "10.3.5.0-24", Value: `{ "PublicIP": "1.1.1.1" }`, ModifiedIndex: 13},
	}

	return &mockSubnetRegistry{
		subnets: &etcd.Node{
			Nodes: subnodes,
		},
		addCh: make(chan string),
		delCh: make(chan string),
		index: 14,
		ttl:   ttlOverride,
	}
}

func (msr *mockSubnetRegistry) getConfig(ctx context.Context, network string) (*etcd.Response, error) {
	return &etcd.Response{
		EtcdIndex: msr.index,
		Node: &etcd.Node{
			Value: `{ "Network": "10.3.0.0/16", "SubnetMin": "10.3.1.0", "SubnetMax": "10.3.5.0" }`,
		},
	}, nil
}

func (msr *mockSubnetRegistry) getSubnets(ctx context.Context, network string) (*etcd.Response, error) {
	return &etcd.Response{
		Node:      msr.subnets,
		EtcdIndex: msr.index,
	}, nil
}

func (msr *mockSubnetRegistry) createSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error) {
	msr.index += 1

	if msr.ttl > 0 {
		ttl = msr.ttl
	}

	// add squared durations :)
	exp := time.Now().Add(time.Duration(ttl) * time.Second)

	node := &etcd.Node{
		Key:           sn,
		Value:         data,
		ModifiedIndex: msr.index,
		Expiration:    &exp,
	}

	msr.subnets.Nodes = append(msr.subnets.Nodes, node)

	return &etcd.Response{
		Node:      node,
		EtcdIndex: msr.index,
	}, nil
}

func (msr *mockSubnetRegistry) updateSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error) {
	msr.index += 1

	// add squared durations :)
	exp := time.Now().Add(time.Duration(ttl) * time.Second)

	for _, n := range msr.subnets.Nodes {
		if n.Key == sn {
			n.Value = data
			n.ModifiedIndex = msr.index
			n.Expiration = &exp

			return &etcd.Response{
				Node:      n,
				EtcdIndex: msr.index,
			}, nil
		}
	}

	return nil, fmt.Errorf("Subnet not found")
}

func (msr *mockSubnetRegistry) watchSubnets(ctx context.Context, network string, since uint64) (*etcd.Response, error) {
	var sn string

	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case sn = <-msr.addCh:
		n := etcd.Node{
			Key:           sn,
			Value:         `{"PublicIP": "1.1.1.1"}`,
			ModifiedIndex: msr.index,
		}
		msr.subnets.Nodes = append(msr.subnets.Nodes, &n)
		return &etcd.Response{
			Action: "add",
			Node:   &n,
		}, nil

	case sn = <-msr.delCh:
		for i, n := range msr.subnets.Nodes {
			if n.Key == sn {
				msr.subnets.Nodes[i] = msr.subnets.Nodes[len(msr.subnets.Nodes)-1]
				msr.subnets.Nodes = msr.subnets.Nodes[:len(msr.subnets.Nodes)-2]
				return &etcd.Response{
					Action: "expire",
					Node:   n,
				}, nil
			}
		}
		return nil, fmt.Errorf("Subnet (%s) to delete was not found: ", sn)
	}
}

func (msr *mockSubnetRegistry) hasSubnet(sn string) bool {
	for _, n := range msr.subnets.Nodes {
		if n.Key == sn {
			return true
		}
	}
	return false
}

func TestAcquireLease(t *testing.T) {
	msr := newMockSubnetRegistry(0)
	sm := newEtcdManager(msr)

	extIP, _ := ip.ParseIP4("1.2.3.4")
	attrs := LeaseAttrs{
		PublicIP: extIP,
	}

	l, err := sm.AcquireLease(context.Background(), "", &attrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	if l.Subnet.String() != "10.3.3.0/24" {
		t.Fatal("Subnet mismatch: expected 10.3.3.0/24, got: ", l.Subnet)
	}

	// Acquire again, should reuse
	if l, err = sm.AcquireLease(context.Background(), "", &attrs); err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	if l.Subnet.String() != "10.3.3.0/24" {
		t.Fatal("Subnet mismatch: expected 10.3.3.0/24, got: ", l.Subnet)
	}
}

func TestWatchLeaseAdded(t *testing.T) {
	msr := newMockSubnetRegistry(0)
	sm := newEtcdManager(msr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan []Event)
	go WatchLeases(ctx, sm, "", events)

	expected := "10.3.3.0-24"
	msr.addCh <- expected

	evtBatch, ok := <-events
	if !ok {
		t.Fatalf("WatchSubnets did not publish")
	}

	if len(evtBatch) != 1 {
		t.Fatalf("WatchSubnets produced wrong sized event batch")
	}

	evt := evtBatch[0]

	if evt.Type != SubnetAdded {
		t.Fatalf("WatchSubnets produced wrong event type")
	}

	actual := evt.Lease.Key()
	if actual != expected {
		t.Errorf("WatchSubnet produced wrong subnet: expected %s, got %s", expected, actual)
	}
}

func TestWatchLeaseRemoved(t *testing.T) {
	msr := newMockSubnetRegistry(0)
	sm := newEtcdManager(msr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan []Event)
	go WatchLeases(ctx, sm, "", events)

	expected := "10.3.4.0-24"
	msr.delCh <- expected

	evtBatch, ok := <-events
	if !ok {
		t.Fatalf("WatchSubnets did not publish")
	}

	if len(evtBatch) != 1 {
		t.Fatalf("WatchSubnets produced wrong sized event batch")
	}

	evt := evtBatch[0]

	if evt.Type != SubnetRemoved {
		t.Fatalf("WatchSubnets produced wrong event type")
	}

	actual := evt.Lease.Key()
	if actual != expected {
		t.Errorf("WatchSubnet produced wrong subnet: expected %s, got %s", expected, actual)
	}
}

type leaseData struct {
	Dummy string
}

func TestRenewLease(t *testing.T) {
	msr := newMockSubnetRegistry(1)
	sm := newEtcdManager(msr)

	// Create LeaseAttrs
	extIP, _ := ip.ParseIP4("1.2.3.4")
	attrs := LeaseAttrs{
		PublicIP:    extIP,
		BackendType: "vxlan",
	}

	ld, err := json.Marshal(&leaseData{Dummy: "test string"})
	if err != nil {
		t.Fatalf("Failed to marshal leaseData: %v", err)
	}
	attrs.BackendData = json.RawMessage(ld)

	// Acquire lease
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l, err := sm.AcquireLease(ctx, "", &attrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	go LeaseRenewer(ctx, sm, "", l)

	fmt.Println("Waiting for lease to pass original expiration")
	time.Sleep(2 * time.Second)

	// check that it's still good
	for _, n := range msr.subnets.Nodes {
		if n.Key == l.Subnet.StringSep(".", "-") {
			if n.Expiration.Before(time.Now()) {
				t.Error("Failed to renew lease: expiration did not advance")
			}
			a := LeaseAttrs{}
			if err := json.Unmarshal([]byte(n.Value), &a); err != nil {
				t.Errorf("Failed to JSON-decode LeaseAttrs: %v", err)
				return
			}
			if !reflect.DeepEqual(a, attrs) {
				t.Errorf("LeaseAttrs changed: was %#v, now %#v", attrs, a)
			}
			return
		}
	}

	t.Fatalf("Failed to find acquired lease")
}
