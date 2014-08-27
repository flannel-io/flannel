package subnet

import (
	"fmt"
	"testing"
	"time"

	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"

	"github.com/coreos/rudder/pkg/ip"
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

func (msr *mockSubnetRegistry) getConfig() (*etcd.Response, error) {
	return &etcd.Response{
		EtcdIndex: msr.index,
		Node: &etcd.Node{
			Value: `{ "Network": "10.3.0.0/16", "SubnetMin": "10.3.1.0", "SubnetMax": "10.3.5.0" }`,
		},
	}, nil
}

func (msr *mockSubnetRegistry) getSubnets() (*etcd.Response, error) {
	return &etcd.Response{
		Node:      msr.subnets,
		EtcdIndex: msr.index,
	}, nil
}

func (msr *mockSubnetRegistry) createSubnet(sn, data string, ttl uint64) (*etcd.Response, error) {
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

func (msr *mockSubnetRegistry) updateSubnet(sn, data string, ttl uint64) (*etcd.Response, error) {

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

func (msr *mockSubnetRegistry) watchSubnets(since uint64, stop chan bool) (*etcd.Response, error) {
	var sn string

	select {
	case <-stop:
		return nil, nil

	case sn = <-msr.addCh:
		n := etcd.Node{
			Key:           sn,
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
	sm, err := newSubnetManager(msr)
	if err != nil {
		t.Fatalf("Failed to create subnet manager: %s", err)
	}

	ip, _ := ip.ParseIP4("1.2.3.4")
	data := `{ "PublicIP": "1.2.3.4" }`

	sn, err := sm.AcquireLease(ip, data)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	if sn.String() != "10.3.3.0/24" {
		t.Fatal("Subnet mismatch: expected 10.3.3.0/24, got: ", sn)
	}

	// Acquire again, should reuse
	if sn, err = sm.AcquireLease(ip, data); err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	if sn.String() != "10.3.3.0/24" {
		t.Fatal("Subnet mismatch: expected 10.3.3.0/24, got: ", sn)
	}
}

func TestWatchLeaseAdded(t *testing.T) {
	msr := newMockSubnetRegistry(0)
	sm, err := newSubnetManager(msr)
	if err != nil {
		t.Fatalf("Failed to create subnet manager: %s", err)
	}

	events := make(chan EventBatch)
	sm.Start(events)

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

	actual := evt.Lease.Network.StringSep(".", "-")
	if actual != expected {
		t.Errorf("WatchSubnet produced wrong subnet: expected %s, got %s", expected, actual)
	}

	sm.Stop()
}

func TestWatchLeaseRemoved(t *testing.T) {
	msr := newMockSubnetRegistry(0)
	sm, err := newSubnetManager(msr)
	if err != nil {
		t.Fatalf("Failed to create subnet manager: %s", err)
	}

	events := make(chan EventBatch)
	sm.Start(events)

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

	actual := evt.Lease.Network.StringSep(".", "-")
	if actual != expected {
		t.Errorf("WatchSubnet produced wrong subnet: expected %s, got %s", expected, actual)
	}

	sm.Stop()
}

func TestRenewLease(t *testing.T) {
	msr := newMockSubnetRegistry(1)
	sm, err := newSubnetManager(msr)
	if err != nil {
		t.Fatalf("Failed to create subnet manager: %s", err)
	}

	ip, _ := ip.ParseIP4("1.2.3.4")
	data := `{ "PublicIP": "1.2.3.4" }`

	sn, err := sm.AcquireLease(ip, data)
	if err != nil {
		t.Fatal("AcquireLease failed: ", err)
	}

	events := make(chan EventBatch)
	sm.Start(events)

	fmt.Println("Waiting for lease to pass original expiration")
	time.Sleep(2 * time.Second)

	// check that it's still good
	for _, n := range msr.subnets.Nodes {
		if n.Key == sn.StringSep(".", "-") {
			if n.Expiration.Before(time.Now()) {
				t.Fatalf("Failed to renew lease")
			}
			return
		}
	}

	t.Fatalf("Failed to find acquired lease")
}
