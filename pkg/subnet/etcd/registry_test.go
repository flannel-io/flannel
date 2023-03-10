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
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	. "github.com/flannel-io/flannel/pkg/subnet"
	etcd "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/integration"
	"golang.org/x/net/context"
)

func newTestEtcdRegistry(t *testing.T, ctx context.Context, client *etcd.Client) (Registry, etcd.KV) {
	cfg := &EtcdConfig{
		Endpoints: []string{"http://127.0.0.1:4001", "http://127.0.0.1:2379"},
		Prefix:    "/coreos.com/network",
	}

	r, err := newEtcdSubnetRegistry(ctx, cfg,
		func(ctx context.Context, c *EtcdConfig) (*etcd.Client, etcd.KV, error) {
			return client, client.KV, nil
		},
	)
	if err != nil {
		t.Fatal("Failed to create etcd subnet registry")
	}

	return r, r.(*etcdSubnetRegistry).kvApi
}

func watchSubnets(t *testing.T, r Registry, ctx context.Context, sn ip.IP4Net, nextIndex int64, result chan error) {
	type leaseEvent struct {
		etype  EventType
		subnet ip.IP4Net
		found  bool
	}
	expectedEvents := []leaseEvent{
		{EventAdded, sn, false},
		{EventRemoved, sn, false},
	}

	receiver := make(chan []LeaseWatchResult)
	numFound := 0

	go func() {
		err := r.watchSubnets(ctx, receiver, nextIndex)
		if err != nil {
			result <- errNoWatchChannel
			return
		}
	}()

	for watchResults := range receiver {
		for _, wr := range watchResults {
			for _, evt := range wr.Events {
				for _, exp := range expectedEvents {
					if evt.Type != exp.etype {
						continue
					}
					if exp.found == true {
						result <- fmt.Errorf("Subnet event type already found: %v", exp)
						return
					}
					if !evt.Lease.Subnet.Equal(exp.subnet) {
						result <- fmt.Errorf("Subnet event lease %v mismatch (expected %v)", evt.Lease.Subnet, exp.subnet)
					}
					exp.found = true
					numFound += 1
				}
				if numFound == len(expectedEvents) {
					// All done; success
					result <- nil
					return
				}
			}

		}

	}
}

func TestEtcdRegistry(t *testing.T) {
	integration.BeforeTestExternal(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	client := clus.RandClient()

	ctx, _ := context.WithCancel(context.Background())

	r, kvApi := newTestEtcdRegistry(t, ctx, client)

	_, err := r.getNetworkConfig(ctx)
	if err != errConfigNotFound {
		t.Fatal("Should hit error getting config")
	}

	// Populate etcd with a network
	netKey := "/coreos.com/network/config"
	netValue := "{ \"Network\": \"10.1.0.0/16\", \"Backend\": { \"Type\": \"host-gw\" } }"
	_, err = kvApi.Put(ctx, netKey, netValue)
	if err != nil {
		t.Fatal("Failed to create new entry", err)
	}

	config, err := r.getNetworkConfig(ctx)
	if err != nil {
		t.Fatal("Failed to get network config", err)
	}
	if config != netValue {
		t.Fatal("Failed to match network config")
	}

	sn := ip.IP4Net{
		IP:        ip.MustParseIP4("10.1.5.0"),
		PrefixLen: 24,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	startWg := sync.WaitGroup{}
	startWg.Add(1)
	result := make(chan error, 1)
	go func() {
		startWg.Done()
		watchSubnets(t, r, ctx, sn, 0, result)
		wg.Done()
	}()

	startWg.Wait()
	// Lease a subnet for the network
	attrs := &LeaseAttrs{
		PublicIP: ip.MustParseIP4("1.2.3.4"),
	}
	exp, err := r.createSubnet(ctx, sn, ip.IP6Net{}, attrs, 24*time.Hour)
	if err != nil {
		t.Fatal("Failed to create subnet lease")
	}
	if !exp.After(time.Now()) {
		t.Fatalf("Subnet lease duration %v not in the future", exp)
	}

	// Make sure the lease got created
	resp, err := kvApi.Get(ctx, "/coreos.com/network/subnets/10.1.5.0-24")
	if err != nil {
		t.Fatalf("Failed to verify subnet lease directly in etcd: %v", err)
	}
	if resp == nil || resp.Kvs == nil {
		t.Fatal("Failed to retrive node in subnet lease")
	}

	if len(resp.Kvs) != 1 || !bytes.Equal(resp.Kvs[0].Value, []byte("{\"PublicIP\":\"1.2.3.4\",\"PublicIPv6\":null}")) {
		t.Fatalf("Unexpected subnet lease node %s value %s", resp.Kvs[0].Key, resp.Kvs[0].Value)
	}

	leases, _, err := r.getSubnets(ctx)
	if err != nil {
		t.Fatal("Failed to get Subnets")
	}
	if len(leases) != 1 {
		t.Fatalf("Unexpected number of leases %d (expected 1)", len(leases))
	}
	if !leases[0].Subnet.Equal(sn) {
		t.Fatalf("Mismatched subnet %v (expected %v)", leases[0].Subnet, sn)
	}

	lease, _, err := r.getSubnet(ctx, sn, ip.IP6Net{})
	if lease == nil {
		t.Fatal("Missing subnet lease")
	}
	if err != nil {
		t.Fatal("Failed to get Subnet")
	}

	err = r.deleteSubnet(ctx, sn, ip.IP6Net{})
	if err != nil {
		t.Fatalf("Failed to delete subnet %v: %v", sn, err)
	}

	// Make sure the lease got deleted
	resp, err = kvApi.Get(ctx, "/coreos.com/network/subnets/10.1.5.0-24")
	if err != nil {
		t.Fatal("Failed to get Subnet" + err.Error())
	}
	if len(resp.Kvs) > 0 {
		t.Fatal("Unexpected success getting deleted subnet")
	}

	wg.Wait()

	// Check errors from watch goroutine
	watchResult := <-result
	if watchResult != nil {
		t.Fatalf("Error watching keys: %v", watchResult)
	}

	// TODO: watchSubnet and watchNetworks
}
