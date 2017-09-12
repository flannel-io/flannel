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

package network

import (
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
	"net"
	"reflect"
	"testing"
)

func lease() *subnet.Lease {
	_, net, _ := net.ParseCIDR("192.168.0.0/16")
	return &subnet.Lease{
		Subnet: ip.FromIPNet(net),
	}
}

type MockIPTablesRule struct {
	table    string
	chain    string
	rulespec []string
}

type MockIPTables struct {
	rules []MockIPTablesRule
}

func (mock *MockIPTables) ruleIndex(table string, chain string, rulespec []string) int {
	for i, rule := range mock.rules {
		if rule.table == table && rule.chain == chain && reflect.DeepEqual(rule.rulespec, rulespec) {
			return i
		}
	}
	return -1
}

func (mock *MockIPTables) Delete(table string, chain string, rulespec ...string) error {
	var ruleIndex = mock.ruleIndex(table, chain, rulespec)
	if ruleIndex != -1 {
		mock.rules = append(mock.rules[:ruleIndex], mock.rules[ruleIndex+1:]...)
	}
	return nil
}

func (mock *MockIPTables) Exists(table string, chain string, rulespec ...string) (bool, error) {
	var ruleIndex = mock.ruleIndex(table, chain, rulespec)
	if ruleIndex != -1 {
		return true, nil
	}
	return false, nil
}

func (mock *MockIPTables) AppendUnique(table string, chain string, rulespec ...string) error {
	var ruleIndex = mock.ruleIndex(table, chain, rulespec)
	if ruleIndex == -1 {
		mock.rules = append(mock.rules, MockIPTablesRule{table: table, chain: chain, rulespec: rulespec})
	}
	return nil
}

func TestDeleteRules(t *testing.T) {
	ipt := &MockIPTables{}
	SetupIPMasq(ipt, ip.IP4Net{}, lease())
	if len(ipt.rules) != 4 {
		t.Errorf("Should be 4 rules, there are actually %d: %#v", len(ipt.rules), ipt.rules)
	}
	TeardownIPMasq(ipt, ip.IP4Net{}, lease())
	if len(ipt.rules) != 0 {
		t.Errorf("Should be 0 rules, there are actually %d: %#v", len(ipt.rules), ipt.rules)
	}
}

func TestEnsureRules(t *testing.T) {
	// If any rules are missing, they should be all deleted and recreated in the correct order
	ipt_correct := &MockIPTables{}
	SetupIPMasq(ipt_correct, ip.IP4Net{}, lease())
	// setup a mock instance where we delete some rules and run `EnsureIPMasq`
	ipt_recreate := &MockIPTables{}
	SetupIPMasq(ipt_recreate, ip.IP4Net{}, lease())
	ipt_recreate.rules = ipt_recreate.rules[0:2]
	EnsureIPMasq(ipt_recreate, ip.IP4Net{}, lease())
	if !reflect.DeepEqual(ipt_recreate.rules, ipt_correct.rules) {
		t.Errorf("iptables rules after EnsureIPMasq are incorrected. Expected: %#v, Actual: %#v", ipt_recreate.rules, ipt_correct.rules)
	}
}
