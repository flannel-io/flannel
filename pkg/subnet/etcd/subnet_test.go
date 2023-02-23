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
	"encoding/json"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	. "github.com/flannel-io/flannel/pkg/subnet"
	etcd "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/integration"
	"golang.org/x/net/context"
	log "k8s.io/klog"
)

func initTestRegistry(ctx context.Context, t *testing.T, r Registry, kvApi etcd.KV) {
	// Populate etcd with a network
	netKey := "/coreos.com/network/config"
	netValue := `{ "Network": "10.3.0.0/16", "SubnetMin": "10.3.1.0", "SubnetMax": "10.3.25.0" }`
	_, err := kvApi.Put(ctx, netKey, netValue)
	if err != nil {
		t.Fatal("Failed to create new entry", err)
	}
	attrs := LeaseAttrs{
		PublicIP: ip.MustParseIP4("1.1.1.1"),
	}

	exp := time.Now().Add(24 * time.Hour)

	subnets := []Lease{
		// leases within SubnetMin-SubnetMax range
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.1.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 10},
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.2.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 11},
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.4.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 12},
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.5.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 13},

		// hand created lease outside the range of subnetMin-SubnetMax for testing removal
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.31.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 13},
	}

	for _, lease := range subnets {
		_, err = r.createSubnet(ctx, lease.Subnet, lease.IPv6Subnet, &attrs, time.Until(lease.Expiration))
		if err != nil {
			t.Fatal("Failed to create new entry", err)
		}
	}
}

func newDummyRegistry() *MockSubnetRegistry {
	attrs := LeaseAttrs{
		PublicIP: ip.MustParseIP4("1.1.1.1"),
	}

	exp := time.Time{}

	subnets := []Lease{
		// leases within SubnetMin-SubnetMax range
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.1.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 10},
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.2.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 11},
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.4.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 12},
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.5.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 13},

		// hand created lease outside the range of subnetMin-SubnetMax for testing removal
		{EnableIPv4: true, EnableIPv6: false, Subnet: ip.IP4Net{IP: ip.MustParseIP4("10.3.31.0"), PrefixLen: 24}, IPv6Subnet: ip.IP6Net{}, Attrs: attrs, Expiration: exp, Asof: 13},
	}

	config := `{ "Network": "10.3.0.0/16", "SubnetMin": "10.3.1.0", "SubnetMax": "10.3.25.0" }`
	return NewMockRegistry(config, subnets)
}

func TestAcquireLease(t *testing.T) {
	msr := newDummyRegistry()
	sm := NewMockManager(msr)

	extIaddr, _ := ip.ParseIP4("1.2.3.4")
	attrs := LeaseAttrs{
		PublicIP: extIaddr,
	}

	l, err := sm.AcquireLease(context.Background(), &attrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	if !inAllocatableRange(context.Background(), sm, l.Subnet) {
		t.Fatal("Subnet mismatch: expected 10.3.3.0/24, got: ", l.Subnet)
	}

	// Acquire again, should reuse
	l2, err := sm.AcquireLease(context.Background(), &attrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	if !l.Subnet.Equal(l2.Subnet) {
		t.Fatalf("AcquireLease did not reuse subnet; expected %v, got %v", l.Subnet, l2.Subnet)
	}

	// Test if a previous subnet will be used
	msr2 := newDummyRegistry()
	prevSubnet := ip.IP4Net{IP: ip.MustParseIP4("10.3.6.0"), PrefixLen: 24}
	sm2 := NewMockManagerWithSubnet(msr2, prevSubnet, ip.IP6Net{})
	prev, err := sm2.AcquireLease(context.Background(), &attrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}
	if !prev.Subnet.Equal(prevSubnet) {
		t.Fatalf("AcquireLease did not reuse subnet from previous run; expected %v, got %v", prevSubnet, prev.Subnet)
	}

	// Test that a previous subnet will not be used if it does not match the registry config
	msr3 := newDummyRegistry()
	invalidSubnet := ip.IP4Net{IP: ip.MustParseIP4("10.4.1.0"), PrefixLen: 24}
	sm3 := NewMockManagerWithSubnet(msr3, invalidSubnet, ip.IP6Net{})
	l3, err := sm3.AcquireLease(context.Background(), &attrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}
	if l3.Subnet.Equal(invalidSubnet) {
		t.Fatalf("AcquireLease reused invalid subnet from previous run; reused %v", l3.Subnet)
	}
}

func TestConfigChanged(t *testing.T) {
	msr := newDummyRegistry()
	sm := NewMockManager(msr)

	extIaddr, _ := ip.ParseIP4("1.2.3.4")
	attrs := LeaseAttrs{
		PublicIP: extIaddr,
	}

	l, err := sm.AcquireLease(context.Background(), &attrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	if !inAllocatableRange(context.Background(), sm, l.Subnet) {
		t.Fatal("Acquired subnet outside of valid range: ", l.Subnet)
	}

	// Change config
	config := `{ "Network": "10.4.0.0/16" }`
	err = msr.setConfig(config)
	if err != nil {
		t.Fatal("Failed to set the Config", err)
	}

	// Acquire again, should not reuse
	if l, err = sm.AcquireLease(context.Background(), &attrs); err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	if !inAllocatableRange(context.Background(), sm, l.Subnet) {
		t.Fatal("Acquired subnet outside of valid range: ", l.Subnet)
	}
}

func acquireLease(ctx context.Context, t *testing.T, sm Manager) *Lease {
	extIaddr, _ := ip.ParseIP4("1.2.3.4")
	attrs := LeaseAttrs{
		PublicIP: extIaddr,
	}

	l, err := sm.AcquireLease(ctx, &attrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	return l
}

func TestWatchLeaseAdded(t *testing.T) {
	integration.BeforeTestExternal(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	client := clus.RandClient()

	ctx, _ := context.WithCancel(context.Background())

	r, kvApi := newTestEtcdRegistry(t, ctx, client)
	initTestRegistry(ctx, t, r, kvApi)
	sm := newLocalManager(r, ip.IP4Net{}, ip.IP6Net{}, 60)

	l := acquireLease(ctx, t, sm)

	events := make(chan []Event)
	go WatchLeases(ctx, sm, l, events)

	select {
	case evtBatch := <-events:
		for _, evt := range evtBatch {
			if evt.Lease.Key() == l.Key() {
				t.Errorf("WatchLeases returned our own lease")
			}
		}
	case <-time.After(5 * time.Second):
		log.Info("no event for our own lease received, we're good")
	}

	expected := ip.IP4Net{
		IP:        ip.MustParseIP4("10.3.30.0"),
		PrefixLen: 24,
	}
	// Sanity check to make sure acquired lease is not this.
	// It shouldn't be as SubnetMin/SubnetMax in config is [10.3.1.0/24 to 10.3.25.0/24]
	if l.Subnet.Equal(expected) {
		t.Fatalf("Acquired lease conflicts with one about to create")
	}

	attrs := &LeaseAttrs{
		PublicIP: ip.MustParseIP4("1.1.1.1"),
	}
	_, err := r.createSubnet(ctx, expected, ip.IP6Net{}, attrs, 0)
	if err != nil {
		t.Fatalf("createSubnet filed: %v", err)
	}

	evtBatch := <-events

	if len(evtBatch) != 1 {
		t.Fatalf("WatchLeases produced wrong sized event batch: got %v, expected 1", len(evtBatch))
	}

	evt := evtBatch[0]

	if evt.Type != EventAdded {
		t.Fatalf("WatchLeases produced wrong event type")
	}

	actual := evt.Lease.Subnet
	if !actual.Equal(expected) {
		t.Errorf("WatchSubnet produced wrong subnet: expected %s, got %s", expected, actual)
	}
	log.Info("test complete!")
}

func TestWatchLeaseRemoved(t *testing.T) {
	integration.BeforeTestExternal(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	client := clus.RandClient()

	ctx, _ := context.WithCancel(context.Background())

	r, kvApi := newTestEtcdRegistry(t, ctx, client)
	netKey := "/coreos.com/network/config"
	netValue := `{ "Network": "10.3.0.0/16", "SubnetMin": "10.3.1.0", "SubnetMax": "10.3.25.0" }`
	_, err := kvApi.Put(ctx, netKey, netValue)
	if err != nil {
		t.Fatal("Failed to create new entry", err)
	}
	sm := newLocalManager(r, ip.IP4Net{}, ip.IP6Net{}, 60)

	l := acquireLease(ctx, t, sm)

	events := make(chan []Event)
	go WatchLeases(ctx, sm, l, events)

	// evtBatch := <-events

	// for _, evt := range evtBatch {
	// 	if evt.Lease.Key() == l.Key() {
	// 		t.Errorf("WatchLeases returned our own lease")
	// 	}
	// }

	expected := ip.IP4Net{IP: ip.MustParseIP4("10.3.31.0"), PrefixLen: 24}
	// Sanity check to make sure acquired lease is not this.
	// It shouldn't be as SubnetMin/SubnetMax in config is [10.3.1.0/24 to 10.3.25.0/24]
	if l.Subnet.Equal(expected) {
		t.Fatalf("Acquired lease conflicts with one about to create")
	}
	attrs := LeaseAttrs{
		PublicIP: ip.MustParseIP4("1.1.1.1"),
	}
	_, err = r.createSubnet(ctx, expected, ip.IP6Net{}, &attrs, time.Until(time.Now().Add(3*time.Second)))
	if err != nil {
		t.Errorf("could not create subnet: %s", err)
	}
	//1. check that the subnet was created
	evtBatch := <-events
	if len(evtBatch) != 1 {
		t.Fatalf("WatchLeases produced wrong sized event batch: %#v", evtBatch)
	}
	if len(evtBatch) != 1 {
		t.Fatalf("WatchLeases produced wrong sized event batch: %#v", evtBatch)
	}

	evt := evtBatch[0]

	if evt.Type != EventAdded {
		t.Fatalf("WatchLeases produced wrong event type")
	}

	//2. check that the subnet was deleted after 3 seconds
	evtBatch = <-events
	if len(evtBatch) != 1 {
		t.Fatalf("WatchLeases produced wrong sized event batch: %#v", evtBatch)
	}

	evt = evtBatch[0]

	if evt.Type != EventRemoved {
		t.Fatalf("WatchLeases produced wrong event type")
	}

	actual := evt.Lease.Subnet
	if !actual.Equal(expected) {
		t.Errorf("WatchSubnet produced wrong subnet: expected %s, got %s", expected, actual)
	}
}

func TestCompleteLease(t *testing.T) {
	integration.BeforeTestExternal(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	client := clus.RandClient()

	ctx, _ := context.WithCancel(context.Background())

	r, kvApi := newTestEtcdRegistry(t, ctx, client)
	initTestRegistry(ctx, t, r, kvApi)
	sm := newLocalManager(r, ip.IP4Net{}, ip.IP6Net{}, 60)

	l := acquireLease(ctx, t, sm)

	evts := make(chan Event)

	go func() {
		myLease := l
		WatchLease(ctx, sm, myLease.Subnet, myLease.IPv6Subnet, evts)
	}()

	event := <-evts
	log.Infof("got event: type: %d, subnet: %s", event.Type, event.Lease.Subnet.String())
	if event.Type != EventAdded {
		t.Fatal("WatchLease: wrong event, expected EventAdded")
	} else {
		log.Info("WatchLease: got EventAdded (lease creation)")
	}

	err := sm.RenewLease(ctx, l)
	if err != nil {
		t.Errorf("failed to renew lease: %s", err)
	}

	event = <-evts
	log.Infof("got event: type: %d, subnet: %s", event.Type, event.Lease.Subnet.String())
	if event.Type != EventAdded {
		t.Fatal("WatchLease: wrong event, expected EventAdded")
	} else {
		log.Info("WatchLease: got EventAdded (lease renewal)")
	}

	leaseKey := path.Join("/coreos.com/network/", "subnets", MakeSubnetKey(l.Subnet, ip.IP6Net{}))
	_, err = kvApi.Delete(ctx, leaseKey)
	if err != nil {
		t.Errorf("could not delete lease: %s", err)
	}

	log.Info("lease deleted manually")
	event = <-evts
	log.Infof("got event: type: %d, subnet: %s", event.Type, event.Lease.Subnet.String())
	if event.Type != EventRemoved {
		t.Fatal("WatchLease: wrong event, expected EventRemoved")
	}

}

type leaseData struct {
	Dummy string
}

func TestRenewLease(t *testing.T) {

	integration.BeforeTestExternal(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	client := clus.RandClient()

	ctx, _ := context.WithCancel(context.Background())

	r, kvApi := newTestEtcdRegistry(t, ctx, client)
	netKey := "/coreos.com/network/config"
	netValue := `{ "Network": "10.3.0.0/16", "SubnetMin": "10.3.1.0", "SubnetMax": "10.3.25.0" }`
	_, err := kvApi.Put(ctx, netKey, netValue)
	if err != nil {
		t.Fatal("Failed to create new entry", err)
	}
	sm := newLocalManager(r, ip.IP4Net{}, ip.IP6Net{}, 60)

	// Create LeaseAttrs
	extIaddr, _ := ip.ParseIP4("1.2.3.4")
	expectedAttrs := LeaseAttrs{
		PublicIP:    extIaddr,
		BackendType: "vxlan",
	}

	ld, err := json.Marshal(&leaseData{Dummy: "test string"})
	if err != nil {
		t.Fatalf("Failed to marshal leaseData: %v", err)
	}
	expectedAttrs.BackendData = json.RawMessage(ld)

	// Acquire lease
	l, err := sm.AcquireLease(ctx, &expectedAttrs)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	//wait a bit so that RenewLease has an effect
	time.Sleep(10 * time.Second)
	if err := sm.RenewLease(ctx, l); err != nil {
		t.Fatal("RenewLease failed: ", err)
	}
	//we expect the new lease to have an expiration date in exactly 24h
	acceptableMargin := 5 * time.Second
	expectedExpiration := time.Now().Add(subnetTTL).Round(time.Duration(acceptableMargin))

	etcdResp, err := kvApi.Get(ctx, "/coreos.com/network/subnets", etcd.WithPrefix())
	if err != nil {
		t.Errorf("failed to renew lease: could not read leases: %s", err)
	}
	for _, resp := range etcdResp.Kvs {
		log.Infof("found key: %s", resp.Key)
		sn, _ := ParseSubnetKey(string(resp.Key))
		if sn.Equal(l.Subnet) {

			ttlResp, err := client.TimeToLive(ctx, etcd.LeaseID(resp.Lease))
			if err != nil {
				t.Errorf("Failed to renew lease: could not ")
			}
			leaseExpiration := time.Now().Add(time.Duration(ttlResp.TTL) * time.Second).Round(time.Duration(acceptableMargin))

			if !leaseExpiration.Equal(expectedExpiration) {
				t.Errorf("Failed to renew lease: bad expiration; expected %v, got %v", expectedExpiration, leaseExpiration)
			}
			//check that the value is correct
			attrs := &LeaseAttrs{}
			err = json.Unmarshal(resp.Value, attrs)
			if err != nil {
				t.Error("Failed to renew lease: could not unmarshal attrs")
			}
			if !reflect.DeepEqual(attrs, &expectedAttrs) {
				t.Errorf("LeaseAttrs changed: was %#v, now %#v", expectedAttrs, attrs)
			}
			return
		}
	}

	t.Fatal("Failed to find acquired lease")
}

func inAllocatableRange(ctx context.Context, sm Manager, ipn ip.IP4Net) bool {
	cfg, err := sm.GetNetworkConfig(ctx)
	if err != nil {
		panic(err)
	}

	return ipn.IP >= cfg.SubnetMin || ipn.IP <= cfg.SubnetMax
}
