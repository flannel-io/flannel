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
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
)

type IPTablesRule struct {
	Table    string
	Action   string
	Chain    string
	Rulespec []string
}

type TrafficManager interface {
	CreateIP4Chain(table, chain string)
	CreateIP6Chain(table, chain string)
	MasqRules(cluster_cidrs []ip.IP4Net, lease *lease.Lease) []IPTablesRule
	MasqIP6Rules(cluster_cidrs []ip.IP6Net, lease *lease.Lease) []IPTablesRule
	ForwardRules(flannelNetwork string) []IPTablesRule
	SetupAndEnsureIP4Tables(getRules func() []IPTablesRule, resyncPeriod int)
	SetupAndEnsureIP6Tables(getRules func() []IPTablesRule, resyncPeriod int)
	DeleteIP4Tables(rules []IPTablesRule) error
	DeleteIP6Tables(rules []IPTablesRule) error
}
