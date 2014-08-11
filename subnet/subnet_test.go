package subnet

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/coreos-inc/kolach/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"

	"github.com/coreos-inc/kolach/pkg"
)

type mockSubnetRegistry struct {
	subnets *etcd.Node
	ch      chan string
	index   uint64
}

func newMockSubnetRegistry(ch chan string) *mockSubnetRegistry {
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
		ch:    ch,
		index: 14,
	}
}

func (msr *mockSubnetRegistry) getConfig() (*etcd.Response, error) {
	return &etcd.Response{
		EtcdIndex: msr.index,
		Node: &etcd.Node{
			Value: `{ "Network": "10.3.0.0/16", "FirstIP": "10.3.1.0", "LastIP": "10.3.5.0" }`,
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

	node := &etcd.Node{
		Key:           sn,
		Value:         data,
		ModifiedIndex: msr.index,
		Expiration:    &exp,
	}

	return &etcd.Response{
		Node:      node,
		EtcdIndex: msr.index,
	}, nil
}

func (msr *mockSubnetRegistry) watchSubnets(since uint64, stop chan bool) (*etcd.Response, error) {
	for {
		var sn string
		select {
		case <-stop:
			return nil, nil
		case sn = <-msr.ch:
			n := etcd.Node{
				Key:           sn,
				ModifiedIndex: msr.index,
			}
			msr.subnets.Nodes = append(msr.subnets.Nodes, &n)
			return &etcd.Response{Node: &n}, nil
		}
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

func netIPNetToString(n *net.IPNet) string {
	return strings.Replace(n.String(), "/", "-", 1)
}

func TestAcquireLease(t *testing.T) {
	msr := newMockSubnetRegistry(nil)
	sm, err := newSubnetManager(msr)
	if err != nil {
		t.Fatalf("Failed to create subnet manager: %s", err)
	}

	ip, _ := pkg.ParseIP4("1.2.3.4")
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

func TestWatchLeases(t *testing.T) {
	msr := newMockSubnetRegistry(make(chan string))
	sm, err := newSubnetManager(msr)
	if err != nil {
		t.Fatalf("Failed to create subnet manager: %s", err)
	}

	ip, _ := pkg.ParseIP4("1.2.3.4")
	data := `{ "PublicIP": "1.2.3.4" }`

	_, err = sm.AcquireLease(ip, data)
	if err != nil {
		t.Fatalf("RegisterSubnet failed: %s", err)
	}

	events := make(chan EventBatch)
	sm.Start(events)

	<-events

	var expected string
	for i := 1; i <= 9; i++ {
		expected = fmt.Sprintf("10.3.%d.0-24", i)
		if !msr.hasSubnet(expected) {
			msr.ch <- expected
			break
		}
	}

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
