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

package nftables

import (
	"context"
	"sync"

	log "k8s.io/klog/v2"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/flannel-io/flannel/pkg/trafficmngr"
)

type NFTablesManager struct {
}

func (nftm *NFTablesManager) Init(ctx context.Context, wg *sync.WaitGroup) error {
	log.Info("Starting flannel in windows mode...")
	return nil
}

func (nftm *NFTablesManager) SetupAndEnsureForwardRules(ctx context.Context,
	flannelIPv4Network ip.IP4Net, flannelIPv6Network ip.IP6Net, resyncPeriod int) {
}

func (nftm *NFTablesManager) SetupAndEnsureMasqRules(ctx context.Context, flannelIPv4Net, prevSubnet, prevNetwork ip.IP4Net,
	flannelIPv6Net, prevIPv6Subnet, prevIPv6Network ip.IP6Net,
	currentlease *lease.Lease,
	resyncPeriod int) error {
	log.Warning(trafficmngr.ErrUnimplemented)
	return nil
}
