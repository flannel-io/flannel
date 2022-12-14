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
//go:build !windows
// +build !windows

package iptables

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/subnet"
)

func lease() *subnet.Lease {
	_, ipv6Net, _ := net.ParseCIDR("fc00::/48")
	_, net, _ := net.ParseCIDR("192.168.0.0/16")
	return &subnet.Lease{
		Subnet:     ip.FromIPNet(net),
		IPv6Subnet: ip.FromIP6Net(ipv6Net),
	}
}

type MockIPTables struct {
	rules    []IPTablesRule
	t        *testing.T
	failures map[string]*MockIPTablesError
}

type MockIPTablesRestore struct {
	t     *testing.T
	rules []IPTablesRestoreRules
}

type MockIPTablesError struct {
	notExist bool
}

func (mock *MockIPTablesError) IsNotExist() bool {
	return mock.notExist
}

func (mock *MockIPTablesError) Error() string {
	return fmt.Sprintf("IsNotExist: %v", !mock.notExist)
}

func (mock *MockIPTablesRestore) ApplyFully(rules IPTablesRestoreRules) error {
	mock.rules = []IPTablesRestoreRules{rules}
	return nil
}

func (mock *MockIPTablesRestore) ApplyWithoutFlush(rules IPTablesRestoreRules) error {
	mock.rules = append(mock.rules, rules)
	return nil
}

func (mock *MockIPTables) ruleIndex(table string, chain string, rulespec []string) int {
	for i, rule := range mock.rules {
		if rule.table == table && rule.chain == chain && reflect.DeepEqual(rule.rulespec, rulespec) {
			return i
		}
	}
	return -1
}

func (mock *MockIPTables) ChainExists(table, chain string) (bool, error) {
	return true, nil
}

func (mock *MockIPTables) ClearChain(table, chain string) error {
	return nil
}

func (mock *MockIPTables) Delete(table string, chain string, rulespec ...string) error {
	var ruleIndex = mock.ruleIndex(table, chain, rulespec)
	key := table + chain + strings.Join(rulespec, "")
	reason := mock.failures[key]
	if reason != nil {
		return reason
	}

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
		mock.rules = append(mock.rules, IPTablesRule{table: table, chain: chain, rulespec: rulespec})
	}
	return nil
}

func TestDeleteRules(t *testing.T) {
	ipt := &MockIPTables{t: t}
	iptr := &MockIPTablesRestore{t: t}
	baseRules := MasqRules([]ip.IP4Net{{
		IP:        ip.MustParseIP4("10.0.1.0"),
		PrefixLen: 16,
	}}, lease())
	expectedRules := expectedTearDownIPTablesRestoreRules(baseRules)

	err := ipTablesBootstrap(ipt, iptr, baseRules)
	if err != nil {
		t.Error("Error bootstrapping up iptables")
	}
	err = setupIPTables(ipt, baseRules)
	if err != nil {
		t.Error("Error setting up iptables")
	}
	if len(ipt.rules) != 7 {
		t.Errorf("Should be 7 masqRules, there are actually %d: %#v", len(ipt.rules), ipt.rules)
	}

	iptr.rules = []IPTablesRestoreRules{}
	err = teardownIPTables(ipt, iptr, baseRules)
	if err != nil {
		t.Error("Error tearing down iptables")
	}
	if !reflect.DeepEqual(expectedRules, iptr.rules) {
		t.Errorf("Incorrect restores rules, Expected: %#v, Actual: %#v", expectedRules, iptr.rules)
	}

}

func TestDeleteMoreRules(t *testing.T) {
	ipt := &MockIPTables{}
	iptr := &MockIPTablesRestore{}

	baseRules := []IPTablesRule{
		{"filter", "-A", "INPUT", []string{"-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"}},
		{"filter", "-A", "INPUT", []string{"-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"}},
		{"nat", "-A", "POSTROUTING", []string{"-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"}},
		{"nat", "-A", "POSTROUTING", []string{"-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"}},
	}

	expectedRules := IPTablesRestoreRules{
		"filter": []IPTablesRestoreRuleSpec{
			IPTablesRestoreRuleSpec{"-D", "INPUT", "-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"},
			IPTablesRestoreRuleSpec{"-D", "INPUT", "-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"},
		},
		"nat": []IPTablesRestoreRuleSpec{
			IPTablesRestoreRuleSpec{"-D", "POSTROUTING", "-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"},
			IPTablesRestoreRuleSpec{"-D", "POSTROUTING", "-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"},
		},
	}

	err := ipTablesBootstrap(ipt, iptr, baseRules)
	if err != nil {
		t.Error("Error bootstrapping up iptables")
	}
	err = setupIPTables(ipt, baseRules)
	if err != nil {
		t.Error("Error setting up iptables")
	}
	if len(ipt.rules) != 4 {
		t.Errorf("Should be 4 masqRules, there are actually %d: %#v", len(ipt.rules), ipt.rules)
	}

	iptr.rules = []IPTablesRestoreRules{}
	err = teardownIPTables(ipt, iptr, baseRules)
	if err != nil {
		t.Error("Error tearing down iptables")
	}
	if !reflect.DeepEqual(iptr.rules, []IPTablesRestoreRules{expectedRules}) {
		t.Errorf("Incorrect restores rules, Expected: %#v, Actual: %#v", expectedRules, iptr.rules)
	}
}

func TestBootstrapRules(t *testing.T) {
	iptr := &MockIPTablesRestore{}
	ipt := &MockIPTables{}

	baseRules := []IPTablesRule{
		{"filter", "-A", "INPUT", []string{"-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"}},
		{"filter", "-A", "INPUT", []string{"-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"}},
		{"nat", "-A", "POSTROUTING", []string{"-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"}},
		{"nat", "-A", "POSTROUTING", []string{"-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"}},
	}

	err := ipTablesBootstrap(ipt, iptr, baseRules)
	if err != nil {
		t.Error("Error bootstrapping up iptables")
	}
	// Ensure iptable mock has rules too
	err = setupIPTables(ipt, baseRules)
	if err != nil {
		t.Error("Error setting up iptables")
	}

	expectedRules := IPTablesRestoreRules{
		"filter": []IPTablesRestoreRuleSpec{
			{"-A", "INPUT", "-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"},
			{"-A", "INPUT", "-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"},
		},
		"nat": []IPTablesRestoreRuleSpec{
			{"-A", "POSTROUTING", "-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"},
			{"-A", "POSTROUTING", "-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"},
		},
	}

	if !reflect.DeepEqual(iptr.rules, []IPTablesRestoreRules{expectedRules}) {
		t.Errorf("iptables masqRules after ensureIPTables are incorrected. Expected: %#v, Actual: %#v", expectedRules, iptr.rules)
	}

	iptr.rules = []IPTablesRestoreRules{}

	expectedRules = IPTablesRestoreRules{
		"filter": []IPTablesRestoreRuleSpec{
			{"-D", "INPUT", "-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"},
			{"-A", "INPUT", "-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"},
			{"-D", "INPUT", "-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"},
			{"-A", "INPUT", "-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"},
		},
		"nat": []IPTablesRestoreRuleSpec{
			{"-D", "POSTROUTING", "-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"},
			{"-A", "POSTROUTING", "-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"},
			{"-D", "POSTROUTING", "-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"},
			{"-A", "POSTROUTING", "-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"},
		},
	}
	// Re-run ensure has new operations
	err = ipTablesBootstrap(ipt, iptr, baseRules)
	if err != nil {
		t.Error("Error bootstrapping up iptables")
	}
	if !reflect.DeepEqual(iptr.rules, []IPTablesRestoreRules{expectedRules}) {
		t.Errorf("iptables masqRules after ensureIPTables are incorrected. Expected: %#v, Actual: %#v", expectedRules, iptr.rules)
	}
}

func TestDeleteIP6Rules(t *testing.T) {
	ipt := &MockIPTables{}
	iptr := &MockIPTablesRestore{}

	baseRules := IP6Rules(ip.IP6Net{}, lease())

	// expect to have the same DELETE rules
	expectedRules := IP6RestoreDeleteRules(ip.IP6Net{}, lease())

	err := ipTablesBootstrap(ipt, iptr, baseRules)
	if err != nil {
		t.Error("Error bootstrapping up iptables")
	}
	err = setupIPTables(ipt, baseRules)
	if err != nil {
		t.Error("Error setting up iptables")
	}
	if len(ipt.rules) != 4 {
		t.Errorf("Should be 4 masqRules, there are actually %d: %#v", len(ipt.rules), ipt.rules)
	}
	iptr.rules = []IPTablesRestoreRules{}
	err = teardownIPTables(ipt, iptr, baseRules)
	if err != nil {
		t.Error("Error tearing down iptables")
	}
	if !reflect.DeepEqual(iptr.rules, []IPTablesRestoreRules{expectedRules}) {
		t.Errorf("Should be 4 deleted iptables rules, there are actually. Expected: %#v, Actual: %#v", expectedRules, iptr.rules)
	}
}

func TestEnsureRules(t *testing.T) {
	iptr := &MockIPTablesRestore{}
	ipt := &MockIPTables{}

	// Ensure iptable mock has other rules
	otherRules := []IPTablesRule{
		{"nat", "-A", "POSTROUTING", []string{"-A", "POSTROUTING", "-j", "KUBE-POSTROUTING"}},
	}
	err := setupIPTables(ipt, otherRules)
	if err != nil {
		t.Error("Error setting up iptables")
	}

	baseRules := []IPTablesRule{
		{"nat", "-A", "POSTROUTING", []string{"-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"}},
		{"nat", "-A", "POSTROUTING", []string{"-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"}},
	}

	err = ensureIPTables(ipt, iptr, baseRules)
	if err != nil {
		t.Errorf("ensureIPTables should have completed without errors")
	}
	// Ensure iptable mock has rules too
	err = setupIPTables(ipt, baseRules)
	if err != nil {
		t.Error("Error setting up iptables")
	}

	expectedRules := IPTablesRestoreRules{
		"nat": []IPTablesRestoreRuleSpec{
			{"-A", "POSTROUTING", "-s", "127.0.0.1", "-d", "127.0.0.1", "-j", "RETURN"},
			{"-A", "POSTROUTING", "-s", "127.0.0.1", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE", "--random-fully"},
		},
	}

	if !reflect.DeepEqual(iptr.rules, []IPTablesRestoreRules{expectedRules}) {
		t.Errorf("iptables masqRules after ensureIPTables are incorrected. Expected: %#v, Actual: %#v", expectedRules, iptr.rules)
	}

	iptr.rules = []IPTablesRestoreRules{}
	// Re-run ensure no new operations
	err = ensureIPTables(ipt, iptr, baseRules)
	if err != nil {
		t.Errorf("ensureIPTables should have completed without errors")
	}
	if len(iptr.rules) > 0 {
		t.Errorf("iptables masqRules after ensureIPTables are incorrected. Expected: %#v, Actual: %#v", expectedRules, iptr.rules)
	}
}

func TestEnsureIP6Rules(t *testing.T) {
	iptr := &MockIPTablesRestore{}
	ipt := &MockIPTables{}

	// Ensure iptable mock has other rules
	otherRules := []IPTablesRule{
		{"nat", "-A", "POSTROUTING", []string{"-A", "POSTROUTING", "-j", "KUBE-POSTROUTING"}},
	}
	err := setupIPTables(ipt, otherRules)
	if err != nil {
		t.Error("Error setting up iptables")
	}

	baseRules := IP6Rules(ip.IP6Net{}, lease())

	err = ensureIPTables(ipt, iptr, baseRules)
	if err != nil {
		t.Errorf("ensureIPTables should have completed without errors")
	}
	// Ensure iptable mock has rules too
	err = setupIPTables(ipt, baseRules)
	if err != nil {
		t.Error("Error setting up iptables")
	}

	expectedRules := IP6RestoreRules(ip.IP6Net{}, lease())

	if !reflect.DeepEqual(iptr.rules, []IPTablesRestoreRules{expectedRules}) {
		t.Errorf("iptables rules after ensureIPTables are incorrected. Expected: %#v, Actual: %#v", expectedRules, iptr.rules)
	}

	iptr.rules = []IPTablesRestoreRules{}
	// Re-run ensure no new operations
	err = ensureIPTables(ipt, iptr, baseRules)
	if err != nil {
		t.Errorf("ensureIPTables should have completed without errors")
	}
	if len(iptr.rules) > 0 {
		t.Errorf("rules masqRules after ensureIPTables are incorrected. Expected empty, Actual: %#v", iptr.rules)
	}

}

func setupIPTables(ipt IPTables, rules []IPTablesRule) error {
	for _, rule := range rules {
		err := ipt.AppendUnique(rule.table, rule.chain, rule.rulespec...)
		if err != nil {
			return fmt.Errorf("failed to insert IPTables rule: %v", err)
		}
	}

	return nil
}

func expectedTearDownIPTablesRestoreRules(rules []IPTablesRule) []IPTablesRestoreRules {
	tablesRules := IPTablesRestoreRules{}
	for _, rule := range rules {
		if _, ok := tablesRules[rule.table]; !ok {
			tablesRules[rule.table] = []IPTablesRestoreRuleSpec{}
		}
		tablesRules[rule.table] = append(tablesRules[rule.table], append(IPTablesRestoreRuleSpec{"-D", rule.chain}, rule.rulespec...))
	}

	return []IPTablesRestoreRules{tablesRules}
}

func IP6Rules(ipn ip.IP6Net, lease *subnet.Lease) []IPTablesRule {
	n := ipn.String()
	sn := lease.IPv6Subnet.String()

	return []IPTablesRule{
		{"nat", "-A", "POSTROUTING", []string{"-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
		{"nat", "-A", "POSTROUTING", []string{"-s", n, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}},
		{"nat", "-A", "POSTROUTING", []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
		{"nat", "-A", "POSTROUTING", []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}},
	}
}

func IP6RestoreRules(ipn ip.IP6Net, lease *subnet.Lease) IPTablesRestoreRules {
	n := ipn.String()
	sn := lease.IPv6Subnet.String()
	return IPTablesRestoreRules{
		"nat": []IPTablesRestoreRuleSpec{
			{"-A", "POSTROUTING", "-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"},
			{"-A", "POSTROUTING", "-s", n, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"},
			{"-A", "POSTROUTING", "!", "-s", n, "-d", sn, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"},
			{"-A", "POSTROUTING", "!", "-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"},
		},
	}
}

func IP6RestoreDeleteRules(ipn ip.IP6Net, lease *subnet.Lease) IPTablesRestoreRules {
	n := ipn.String()
	sn := lease.IPv6Subnet.String()
	return IPTablesRestoreRules{
		"nat": []IPTablesRestoreRuleSpec{
			{"-D", "POSTROUTING", "-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"},
			{"-D", "POSTROUTING", "-s", n, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"},
			{"-D", "POSTROUTING", "!", "-s", n, "-d", sn, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"},
			{"-D", "POSTROUTING", "!", "-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"},
		},
	}
}
