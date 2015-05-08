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

package remote

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

const expectedNetwork = "10.1.0.0/16"

func TestRemote(t *testing.T) {
	config := fmt.Sprintf(`{"Network": %q}`, expectedNetwork)
	sm := subnet.NewMockManager(1, config)

	addr := "127.0.0.1:9999"

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		RunServer(ctx, sm, addr)
		wg.Done()
	}()

	doTestRemote(ctx, t, addr)

	cancel()
	wg.Wait()
}

func mustParseIP4(s string) ip.IP4 {
	a, err := ip.ParseIP4(s)
	if err != nil {
		panic(err)
	}
	return a
}

func mustParseIP4Net(s string) ip.IP4Net {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return ip.FromIPNet(n)
}

func doTestRemote(ctx context.Context, t *testing.T, remoteAddr string) {
	sm := NewRemoteManager(remoteAddr)

	cfg, err := sm.GetNetworkConfig(ctx, "_")
	if err != nil {
		t.Errorf("GetNetworkConfig failed: %v", err)
	}

	if cfg.Network.String() != expectedNetwork {
		t.Errorf("GetNetworkConfig returned bad network: %v vs %v", cfg.Network, expectedNetwork)
	}

	attrs := &subnet.LeaseAttrs{
		PublicIP: mustParseIP4("1.1.1.1"),
	}
	l, err := sm.AcquireLease(ctx, "_", attrs)
	if err != nil {
		t.Errorf("AcquireLease failed: %v", err)
	}

	if !mustParseIP4Net(expectedNetwork).Contains(l.Subnet.IP) {
		t.Errorf("AcquireLease returned subnet not in network: %v (in %v)", l.Subnet, expectedNetwork)
	}

	if err = sm.RenewLease(ctx, "_", l); err != nil {
		t.Errorf("RenewLease failed: %v", err)
	}

	doTestWatch(t, sm)
}

func doTestWatch(t *testing.T, sm subnet.Manager) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	res := make(chan error)
	barrier := make(chan struct{})

	sm.WatchLeases(ctx, "_", nil)

	var expectedSubnet ip.IP4Net

	go func() {
		wr, err := sm.WatchLeases(ctx, "_", nil)
		if err != nil {
			res <- fmt.Errorf("WatchLeases failed: %v", err)
			return
		}
		if len(wr.Events) > 0 && len(wr.Snapshot) > 0 {
			res <- fmt.Errorf("WatchLeases returned events and snapshots")
			return
		}

		res <- nil
		<-barrier

		wr, err = sm.WatchLeases(ctx, "_", wr.Cursor)
		if err != nil {
			res <- fmt.Errorf("WatchLeases failed: %v", err)
			return
		}
		if len(wr.Events) == 0 {
			res <- fmt.Errorf("WatchLeases returned empty events")
			return
		}

		if wr.Events[0].Type != subnet.SubnetAdded {
			res <- fmt.Errorf("WatchLeases returned event with wrong EventType: %v vs %v", wr.Events[0].Type, subnet.SubnetAdded)
			return
		}

		if !wr.Events[0].Lease.Subnet.Equal(expectedSubnet) {
			res <- fmt.Errorf("WatchLeases returned unexpected subnet: %v vs %v", wr.Events[0].Lease.Subnet, expectedSubnet)
		}

		res <- nil
	}()

	if err := <-res; err != nil {
		t.Fatal(err.Error())
	}

	attrs := &subnet.LeaseAttrs{
		PublicIP: mustParseIP4("1.1.1.2"),
	}
	l, err := sm.AcquireLease(ctx, "_", attrs)
	if err != nil {
		t.Errorf("AcquireLease failed: %v", err)
		return
	}
	if !mustParseIP4Net(expectedNetwork).Contains(l.Subnet.IP) {
		t.Errorf("AcquireLease returned subnet not in network: %v (in %v)", l.Subnet, expectedNetwork)
	}

	expectedSubnet = l.Subnet

	barrier <- struct{}{}
	if err := <-res; err != nil {
		t.Fatal(err.Error())
	}
}
