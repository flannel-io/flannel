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

func newDummyRegistry(ttlOverride uint64) *mockSubnetRegistry {
	subnets := []*etcd.Node{
		&etcd.Node{Key: "10.3.1.0-24", Value: `{ "PublicIP": "1.1.1.1" }`, ModifiedIndex: 10},
		&etcd.Node{Key: "10.3.2.0-24", Value: `{ "PublicIP": "1.1.1.1" }`, ModifiedIndex: 11},
		&etcd.Node{Key: "10.3.4.0-24", Value: `{ "PublicIP": "1.1.1.1" }`, ModifiedIndex: 12},
		&etcd.Node{Key: "10.3.5.0-24", Value: `{ "PublicIP": "1.1.1.1" }`, ModifiedIndex: 13},
	}

	config := `{ "Network": "10.3.0.0/16", "SubnetMin": "10.3.1.0", "SubnetMax": "10.3.5.0" }`
	return newMockRegistry(ttlOverride, config, subnets)
}

func TestAcquireLease(t *testing.T) {
	msr := newDummyRegistry(1000)
	sm := newEtcdManager(msr)

	extIaddr, _ := ip.ParseIP4("1.2.3.4")
	attrs := LeaseAttrs{
		PublicIP: extIaddr,
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

func TestConfigChanged(t *testing.T) {
	msr := newDummyRegistry(1000)
	sm := newEtcdManager(msr)

	extIaddr, _ := ip.ParseIP4("1.2.3.4")
	attrs := LeaseAttrs{
		PublicIP: extIaddr,
	}

	l, err := sm.AcquireLease(context.Background(), "", &attrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	if l.Subnet.String() != "10.3.3.0/24" {
		t.Fatal("Subnet mismatch: expected 10.3.3.0/24, got: ", l.Subnet)
	}

	// Change config
	config := `{ "Network": "10.4.0.0/16" }`
	msr.setConfig(config)

	// Acquire again, should not reuse
	if l, err = sm.AcquireLease(context.Background(), "", &attrs); err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	newNet := newIP4Net("10.4.0.0", 16)
	if !newNet.Contains(l.Subnet.IP) {
		t.Fatalf("Subnet mismatch: expected within %v, got: %v", newNet, l.Subnet)
	}
}

func newIP4Net(ipaddr string, prefix uint) ip.IP4Net {
	a, err := ip.ParseIP4(ipaddr)
	if err != nil {
		panic("failed to parse ipaddr")
	}
	return ip.IP4Net{
		IP:        a,
		PrefixLen: prefix,
	}
}

func TestWatchLeaseAdded(t *testing.T) {
	msr := newDummyRegistry(0)
	sm := newEtcdManager(msr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan []Event)
	go WatchLeases(ctx, sm, "", events)

	// skip over the initial snapshot
	<-events

	expected := "10.3.3.0-24"
	msr.createSubnet(ctx, "_", expected, `{"PublicIP": "1.1.1.1"}`, 0)

	evtBatch := <-events

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
	msr := newDummyRegistry(0)
	sm := newEtcdManager(msr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan []Event)
	go WatchLeases(ctx, sm, "", events)

	// skip over the initial snapshot
	<-events

	expected := "10.3.4.0-24"
	msr.expireSubnet(expected)

	evtBatch := <-events

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
	msr := newDummyRegistry(1)
	sm := newEtcdManager(msr)

	// Create LeaseAttrs
	extIaddr, _ := ip.ParseIP4("1.2.3.4")
	attrs := LeaseAttrs{
		PublicIP:    extIaddr,
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
