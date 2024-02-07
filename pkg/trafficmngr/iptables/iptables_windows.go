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

package iptables

import (
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/flannel-io/flannel/pkg/trafficmngr"
)

type IPTablesManager struct{}

type IPTables interface {
	AppendUnique(table string, chain string, rulespec ...string) error
	ChainExists(table, chain string) (bool, error)
	ClearChain(table, chain string) error
	Delete(table string, chain string, rulespec ...string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
}

func (iptm IPTablesManager) CreateIP4Chain(table, chain string) { return }
func (iptm IPTablesManager) CreateIP6Chain(table, chain string) { return }
func (iptm IPTablesManager) MasqRules(cluster_cidrs []ip.IP4Net, lease *lease.Lease) []trafficmngr.IPTablesRule {
	return nil
}
func (iptm IPTablesManager) ForwardRules(flannelNetwork string) []trafficmngr.IPTablesRule {
	return nil
}
func teardownIPTables(ipt IPTables, rules []trafficmngr.IPTablesRule) {}
func (iptm IPTablesManager) SetupAndEnsureIP4Tables(getRules func() []trafficmngr.IPTablesRule, resyncPeriod int) {
}
func (iptm IPTablesManager) SetupAndEnsureIP6Tables(getRules func() []trafficmngr.IPTablesRule, resyncPeriod int) {
}
func (iptm IPTablesManager) MasqIP6Rules(cluster_cidrs []ip.IP6Net, lease *lease.Lease) []trafficmngr.IPTablesRule {
	return nil
}
func (iptm IPTablesManager) DeleteIP4Tables(rules []trafficmngr.IPTablesRule) error { return nil }
func (iptm IPTablesManager) DeleteIP6Tables(rules []trafficmngr.IPTablesRule) error { return nil }
