// Copyright 2024 flannel authors
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

package trafficmngr

import (
	"context"
	"errors"
	"sync"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
)

type IPTablesRule struct {
	Table    string
	Action   string
	Chain    string
	Rulespec []string
}

var (
	ErrUnimplemented = errors.New("unimplemented")
)

const KubeProxyMark string = "0x4000/0x4000"

type TrafficManager interface {
	// Initialize the TrafficManager, including the go routine to clean-up when flanneld is closed
	Init(ctx context.Context, wg *sync.WaitGroup) error
	// Install kernel rules to forward the traffic to and from the flannel network range.
	// This is done for IPv4 and/or IPv6 based on whether flannelIPv4Network and flannelIPv6Network are set.
	// SetupAndEnsureForwardRules starts a go routine that
	// rewrites these rules every resyncPeriod seconds if needed
	SetupAndEnsureForwardRules(ctx context.Context, flannelIPv4Network ip.IP4Net, flannelIPv6Network ip.IP6Net, resyncPeriod int)
	// Install kernel rules to setup NATing of packets sent to the flannel interface
	// This is done for IPv4 and/or IPv6 based on whether flannelIPv4Network and flannelIPv6Network are set.
	// prevSubnet,prevNetworks, prevIPv6Subnet, prevIPv6Networks are used
	// to determine whether the existing rules need to be replaced.
	// SetupAndEnsureMasqRules starts a go routine that
	// rewrites these rules every resyncPeriod seconds if needed
	SetupAndEnsureMasqRules(ctx context.Context,
		flannelIPv4Net, prevSubnet, prevNetwork ip.IP4Net,
		flannelIPv6Net, prevIPv6Subnet, prevIPv6Network ip.IP6Net,
		currentlease *lease.Lease,
		resyncPeriod int) error
}
