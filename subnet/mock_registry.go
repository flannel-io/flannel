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
	"fmt"
	"time"

	etcd "github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/etcd/client"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
)

type mockSubnetRegistry struct {
	config  *etcd.Node
	subnets *etcd.Node
	events  chan *etcd.Response
	index   uint64
	ttl     time.Duration
}

func newMockRegistry(ttlOverride time.Duration, config string, initialSubnets []*etcd.Node) *mockSubnetRegistry {
	index := uint64(0)
	for _, n := range initialSubnets {
		if n.ModifiedIndex > index {
			index = n.ModifiedIndex
		}
	}

	return &mockSubnetRegistry{
		config: &etcd.Node{
			Value: config,
		},
		subnets: &etcd.Node{
			Nodes: initialSubnets,
		},
		events: make(chan *etcd.Response, 1000),
		index:  index + 1,
		ttl:    ttlOverride,
	}
}

func (msr *mockSubnetRegistry) getConfig(ctx context.Context, network string) (*etcd.Response, error) {
	return &etcd.Response{
		Index: msr.index,
		Node:  msr.config,
	}, nil
}

func (msr *mockSubnetRegistry) setConfig(config string) {
	msr.config = &etcd.Node{
		Key:   "config",
		Value: config,
	}
}

func (msr *mockSubnetRegistry) getSubnets(ctx context.Context, network string) (*etcd.Response, error) {
	return &etcd.Response{
		Node:  msr.subnets,
		Index: msr.index,
	}, nil
}

func (msr *mockSubnetRegistry) createSubnet(ctx context.Context, network, sn, data string, ttl time.Duration) (*etcd.Response, error) {
	msr.index += 1

	if msr.ttl > 0 {
		ttl = msr.ttl
	}

	exp := time.Now().Add(ttl)

	node := &etcd.Node{
		Key:           sn,
		Value:         data,
		ModifiedIndex: msr.index,
		Expiration:    &exp,
	}

	msr.subnets.Nodes = append(msr.subnets.Nodes, node)
	msr.events <- &etcd.Response{
		Action: "add",
		Node:   node,
	}

	return &etcd.Response{
		Node:  node,
		Index: msr.index,
	}, nil
}

func (msr *mockSubnetRegistry) updateSubnet(ctx context.Context, network, sn, data string, ttl time.Duration) (*etcd.Response, error) {
	msr.index += 1

	exp := time.Now().Add(ttl)

	for _, n := range msr.subnets.Nodes {
		if n.Key == sn {
			n.Value = data
			n.ModifiedIndex = msr.index
			n.Expiration = &exp
			msr.events <- &etcd.Response{
				Action: "add",
				Node:   n,
			}

			return &etcd.Response{
				Node:  n,
				Index: msr.index,
			}, nil
		}
	}

	return nil, fmt.Errorf("Subnet not found")
}

func (msr *mockSubnetRegistry) deleteSubnet(ctx context.Context, network, sn string) (*etcd.Response, error) {
	msr.index += 1

	for i, n := range msr.subnets.Nodes {
		if n.Key == sn {
			msr.subnets.Nodes[i] = msr.subnets.Nodes[len(msr.subnets.Nodes)-1]
			msr.subnets.Nodes = msr.subnets.Nodes[:len(msr.subnets.Nodes)-1]
			msr.events <- &etcd.Response{
				Action: "delete",
				Node:   n,
			}

			return &etcd.Response{
				Node:  n,
				Index: msr.index,
			}, nil
		}
	}

	return nil, fmt.Errorf("Subnet not found")

}

func (msr *mockSubnetRegistry) watchSubnets(ctx context.Context, network string, since uint64) (*etcd.Response, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case r := <-msr.events:
			if r.Node.ModifiedIndex <= since {
				continue
			}
			return r, nil
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

func (msr *mockSubnetRegistry) expireSubnet(sn string) {
	for i, n := range msr.subnets.Nodes {
		if n.Key == sn {
			msr.index += 1
			msr.subnets.Nodes[i] = msr.subnets.Nodes[len(msr.subnets.Nodes)-1]
			msr.subnets.Nodes = msr.subnets.Nodes[:len(msr.subnets.Nodes)-2]
			n.ModifiedIndex = msr.index
			msr.events <- &etcd.Response{
				Action: "expire",
				Node:   n,
			}
			return
		}
	}
}
