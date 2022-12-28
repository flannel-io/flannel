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
)

type IPTables interface {
	AppendUnique(table string, chain string, rulespec ...string) error
	ChainExists(table, chain string) (bool, error)
	ClearChain(table, chain string) error
	Delete(table string, chain string, rulespec ...string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
}

type IPTablesRule struct {
	table    string
	action   string
	chain    string
	rulespec []string
}

func CreateIP4Chain(table, chain string)                                        { return }
func CreateIP6Chain(table, chain string)                                        { return }
func MasqRules(cluster_cidrs []ip.IP4Net, lease *lease.Lease) []IPTablesRule    { return nil }
func ForwardRules(flannelNetwork string) []IPTablesRule                         { return nil }
func teardownIPTables(ipt IPTables, rules []IPTablesRule)                       {}
func SetupAndEnsureIP4Tables(getRules func() []IPTablesRule, resyncPeriod int)  {}
func SetupAndEnsureIP6Tables(getRules func() []IPTablesRule, resyncPeriod int)  {}
func MasqIP6Rules(cluster_cidrs []ip.IP6Net, lease *lease.Lease) []IPTablesRule { return nil }
func DeleteIP4Tables(rules []IPTablesRule) error                                { return nil }
func DeleteIP6Tables(rules []IPTablesRule) error                                { return nil }
