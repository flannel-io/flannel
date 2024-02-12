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

type IPTablesManager struct{}

type IPTables interface {
	AppendUnique(table string, chain string, rulespec ...string) error
	ChainExists(table, chain string) (bool, error)
	ClearChain(table, chain string) error
	Delete(table string, chain string, rulespec ...string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
}

func (iptm IPTablesManager) SetupAndEnsureForwardRules(flannelIPv4Network, flannelIPv6Network string, resyncPeriod int) {
}

func (iptm IPTablesManager) SetupAndEnsureMasqRules(flannelIPv4Net, prevSubnet ip.IP4Net,
	prevNetworks []ip.IP4Net,
	currentlease *lease.Lease,
	flannelIPv6Net, prevIPv6Subnet ip.IP6Net,
	prevIPv6Networks []ip.IP6Net,
	resyncPeriod int) error {
	return nil
}
