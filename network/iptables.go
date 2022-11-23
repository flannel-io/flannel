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

package network

import (
	"fmt"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/subnet"
	log "k8s.io/klog"
)

type IPTables interface {
	AppendUnique(table string, chain string, rulespec ...string) error
	ChainExists(table, chain string) (bool, error)
	ClearChain(table, chain string) error
	Delete(table string, chain string, rulespec ...string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
}

type IPTablesError interface {
	IsNotExist() bool
	Error() string
}

type IPTablesRule struct {
	table    string
	action   string
	chain    string
	rulespec []string
}

const kubeProxyMark string = "0x4000/0x4000"

func MasqRules(ipn ip.IP4Net, lease *subnet.Lease) []IPTablesRule {
	n := ipn.String()
	sn := lease.Subnet.String()
	supports_random_fully := false
	ipt, err := iptables.New()
	if err == nil {
		supports_random_fully = ipt.HasRandomFully()
	}

	if supports_random_fully {
		return []IPTablesRule{
			// This rule ensure that the flannel iptables rules are executed before other rules on the node
			{"nat", "-I", "POSTROUTING", []string{"-m", "comment", "--comment", "flanneld masq", "-j", "FLANNEL-POSTRTG"}},
			// This rule will not masquerade traffic marked by the kube-proxy to avoid double NAT bug on some kernel version
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-m", "mark", "--mark", kubeProxyMark, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// NAT if it's not multicast traffic
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-s", n, "!", "-d", "224.0.0.0/4", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}},
			// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// Masquerade anything headed towards flannel from the host
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}},
		}
	} else {
		return []IPTablesRule{
			// This rule ensure that the flannel iptables rules are executed before other rules on the node
			{"nat", "-I", "POSTROUTING", []string{"-m", "comment", "--comment", "flanneld masq", "-j", "FLANNEL-POSTRTG"}},
			// This rule will not masquerade traffic marked by the kube-proxy to avoid double NAT bug on some kernel version
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-m", "mark", "--mark", kubeProxyMark, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// NAT if it's not multicast traffic
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-s", n, "!", "-d", "224.0.0.0/4", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE"}},
			// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// Masquerade anything headed towards flannel from the host
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE"}},
		}
	}
}

func MasqIP6Rules(ipn ip.IP6Net, lease *subnet.Lease) []IPTablesRule {
	n := ipn.String()
	sn := lease.IPv6Subnet.String()
	supports_random_fully := false
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err == nil {
		supports_random_fully = ipt.HasRandomFully()
	}

	if supports_random_fully {
		return []IPTablesRule{
			// This rule ensure that the flannel iptables rules are executed before other rules on the node
			{"nat", "-I", "POSTROUTING", []string{"-m", "comment", "--comment", "flanneld masq", "-j", "FLANNEL-POSTRTG"}},
			// This rule will not masquerade traffic marked by the kube-proxy to avoid double NAT bug on some kernel version
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-m", "mark", "--mark", kubeProxyMark, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// NAT if it's not multicast traffic
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-s", n, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}},
			// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// Masquerade anything headed towards flannel from the host
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}},
		}
	} else {
		return []IPTablesRule{
			// This rule ensure that the flannel iptables rules are executed before other rules on the node
			{"nat", "-I", "POSTROUTING", []string{"-m", "comment", "--comment", "flanneld masq", "-j", "FLANNEL-POSTRTG"}},
			// This rule will not masquerade traffic marked by the kube-proxy to avoid double NAT bug on some kernel version
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-m", "mark", "--mark", kubeProxyMark, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// NAT if it's not multicast traffic
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"-s", n, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE"}},
			// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			// Masquerade anything headed towards flannel from the host
			{"nat", "-A", "FLANNEL-POSTRTG", []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE"}},
		}
	}
}

func ForwardRules(flannelNetwork string) []IPTablesRule {
	return []IPTablesRule{
		// This rule ensure that the flannel iptables rules are executed before other rules on the node
		{"filter", "-I", "FORWARD", []string{"-m", "comment", "--comment", "flanneld forward", "-j", "FLANNEL-FWD"}},
		// These rules allow traffic to be forwarded if it is to or from the flannel network range.
		{"filter", "-A", "FLANNEL-FWD", []string{"-s", flannelNetwork, "-m", "comment", "--comment", "flanneld forward", "-j", "ACCEPT"}},
		{"filter", "-A", "FLANNEL-FWD", []string{"-d", flannelNetwork, "-m", "comment", "--comment", "flanneld forward", "-j", "ACCEPT"}},
	}
}

func CreateIP4Chain(table, chain string) {
	ipt, err := iptables.New()
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IPTables. iptables binary was not found: %v", err)
		return
	}
	err = ipt.ClearChain(table, chain)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IPTables. Error on creating the chain: %v", err)
		return
	}
}

func CreateIP6Chain(table, chain string) {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IP6Tables. iptables binary was not found: %v", err)
		return
	}
	err = ipt.ClearChain(table, chain)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IP6Tables. Error on creating the chain: %v", err)
		return
	}
}

func ipTablesRulesExist(ipt IPTables, rules []IPTablesRule) (bool, error) {
	for _, rule := range rules {
		if rule.chain == "FLANNEL-FWD" || rule.rulespec[len(rule.rulespec)-1] == "FLANNEL-FWD" {
			chainExist, err := ipt.ChainExists(rule.table, "FLANNEL-FWD")
			if err != nil {
				return false, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				return false, nil
			}
		} else if rule.chain == "FLANNEL-POSTRTG" || rule.rulespec[len(rule.rulespec)-1] == "FLANNEL-POSTRTG" {
			chainExist, err := ipt.ChainExists(rule.table, "FLANNEL-POSTRTG")
			if err != nil {
				return false, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				return false, nil
			}
		}
		exists, err := ipt.Exists(rule.table, rule.chain, rule.rulespec...)
		if err != nil {
			// this shouldn't ever happen
			return false, fmt.Errorf("failed to check rule existence: %v", err)
		}
		if !exists {
			return false, nil
		}
	}

	return true, nil
}

// ipTablesCleanAndBuild create from a list of iptables rules a transaction (as string) for iptables-restore for ordering the rules that effectively running
func ipTablesCleanAndBuild(ipt IPTables, rules []IPTablesRule) (IPTablesRestoreRules, error) {
	tablesRules := IPTablesRestoreRules{}

	// Build append and delete rules
	for _, rule := range rules {
		if rule.chain == "FLANNEL-FWD" || rule.rulespec[len(rule.rulespec)-1] == "FLANNEL-FWD" {
			chainExist, err := ipt.ChainExists(rule.table, "FLANNEL-FWD")
			if err != nil {
				return nil, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				err = ipt.ClearChain(rule.table, "FLANNEL-FWD")
				if err != nil {
					return nil, fmt.Errorf("failed to create rule chain: %v", err)
				}
			}
		} else if rule.chain == "FLANNEL-POSTRTG" || rule.rulespec[len(rule.rulespec)-1] == "FLANNEL-POSTRTG" {
			chainExist, err := ipt.ChainExists(rule.table, "FLANNEL-POSTRTG")
			if err != nil {
				return nil, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				err = ipt.ClearChain(rule.table, "FLANNEL-POSTRTG")
				if err != nil {
					return nil, fmt.Errorf("failed to create rule chain: %v", err)
				}
			}
		}
		exists, err := ipt.Exists(rule.table, rule.chain, rule.rulespec...)
		if err != nil {
			// this shouldn't ever happen
			return nil, fmt.Errorf("failed to check rule existence: %v", err)
		}
		if exists {
			if _, ok := tablesRules[rule.table]; !ok {
				tablesRules[rule.table] = []IPTablesRestoreRuleSpec{}
			}
			// if the rule exists it's safer to delete it and then create them
			tablesRules[rule.table] = append(tablesRules[rule.table], append(IPTablesRestoreRuleSpec{"-D", rule.chain}, rule.rulespec...))
		}
		// with iptables-restore we can ensure that all rules created are in good order and have no external rule between them
		tablesRules[rule.table] = append(tablesRules[rule.table], append(IPTablesRestoreRuleSpec{rule.action, rule.chain}, rule.rulespec...))
	}

	return tablesRules, nil
}

// ipTablesBootstrap init iptables rules using iptables-restore (with some cleaning if some rules already exists)
func ipTablesBootstrap(ipt IPTables, iptRestore IPTablesRestore, rules []IPTablesRule) error {
	tablesRules, err := ipTablesCleanAndBuild(ipt, rules)
	if err != nil {
		// if we can't find iptables or if we can check existing rules, give up and return
		return fmt.Errorf("failed to setup iptables-restore payload: %v", err)
	}

	log.V(6).Infof("trying to run iptables-restore < %+v", tablesRules)

	err = iptRestore.ApplyWithoutFlush(tablesRules)
	if err != nil {
		return fmt.Errorf("failed to apply partial iptables-restore %v", err)
	}

	log.Infof("bootstrap done")

	return nil
}

func SetupAndEnsureIP4Tables(rules []IPTablesRule, resyncPeriod int) {
	ipt, err := iptables.New()
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IPTables. iptables binary was not found: %v", err)
		return
	}
	iptRestore, err := NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		// if we can't find iptables-restore, give up and return
		log.Errorf("Failed to setup IPTables. iptables-restore binary was not found: %v", err)
		return
	}

	err = ipTablesBootstrap(ipt, iptRestore, rules)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to bootstrap IPTables: %v", err)
	}

	defer func() {
		err := teardownIPTables(ipt, iptRestore, rules)
		if err != nil {
			log.Errorf("Failed to tear down IPTables: %v", err)
		}
	}()

	for {
		// Ensure that all the iptables rules exist every 5 seconds
		if err := ensureIPTables(ipt, iptRestore, rules); err != nil {
			log.Errorf("Failed to ensure iptables rules: %v", err)
		}

		time.Sleep(time.Duration(resyncPeriod) * time.Second)
	}
}

func SetupAndEnsureIP6Tables(rules []IPTablesRule, resyncPeriod int) {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IP6Tables. iptables binary was not found: %v", err)
		return
	}
	iptRestore, err := NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup iptables-restore: %v", err)
		return
	}

	err = ipTablesBootstrap(ipt, iptRestore, rules)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to bootstrap IPTables: %v", err)
	}

	defer func() {
		err := teardownIPTables(ipt, iptRestore, rules)
		if err != nil {
			log.Errorf("Failed to tear down IPTables: %v", err)
		}
	}()

	for {
		// Ensure that all the iptables rules exist every 5 seconds
		if err := ensureIPTables(ipt, iptRestore, rules); err != nil {
			log.Errorf("Failed to ensure iptables rules: %v", err)
		}

		time.Sleep(time.Duration(resyncPeriod) * time.Second)
	}
}

// DeleteIP4Tables delete specified iptables rules
func DeleteIP4Tables(rules []IPTablesRule) error {
	ipt, err := iptables.New()
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IPTables. iptables binary was not found: %v", err)
		return err
	}
	iptRestore, err := NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup iptables-restore: %v", err)
		return err
	}
	err = teardownIPTables(ipt, iptRestore, rules)
	if err != nil {
		log.Errorf("Failed to teardown iptables: %v", err)
		return err
	}
	return nil
}

// DeleteIP6Tables delete specified iptables rules
func DeleteIP6Tables(rules []IPTablesRule) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IP6Tables. iptables binary was not found: %v", err)
		return err
	}

	iptRestore, err := NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup iptables-restore: %v", err)
		return err
	}
	err = teardownIPTables(ipt, iptRestore, rules)
	if err != nil {
		log.Errorf("Failed to teardown iptables: %v", err)
		return err
	}
	return nil
}

func ensureIPTables(ipt IPTables, iptRestore IPTablesRestore, rules []IPTablesRule) error {
	exists, err := ipTablesRulesExist(ipt, rules)
	if err != nil {
		return fmt.Errorf("error checking rule existence: %v", err)
	}
	if exists {
		// if all the rules already exist, no need to do anything
		return nil
	}
	// Otherwise, teardown all the rules and set them up again
	// We do this because the order of the rules is important
	log.Info("Some iptables rules are missing; deleting and recreating rules")
	err = ipTablesBootstrap(ipt, iptRestore, rules)
	if err != nil {
		// if we can't find iptables, give up and return
		return fmt.Errorf("error setting up rules: %v", err)
	}
	return nil
}

func teardownIPTables(ipt IPTables, iptr IPTablesRestore, rules []IPTablesRule) error {
	tablesRules := IPTablesRestoreRules{}

	// Build delete rules to a transaction for iptables restore
	for _, rule := range rules {
		if rule.chain == "FLANNEL-FWD" || rule.rulespec[len(rule.rulespec)-1] == "FLANNEL-FWD" {
			chainExists, err := ipt.ChainExists(rule.table, "FLANNEL-FWD")
			if err != nil {
				// this shouldn't ever happen
				return fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExists {
				continue
			}
		} else if rule.chain == "FLANNEL-POSTRTG" || rule.rulespec[len(rule.rulespec)-1] == "FLANNEL-POSTRTG" {
			chainExists, err := ipt.ChainExists(rule.table, "FLANNEL-POSTRTG")
			if err != nil {
				// this shouldn't ever happen
				return fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExists {
				continue
			}
		}
		exists, err := ipt.Exists(rule.table, rule.chain, rule.rulespec...)
		if err != nil {
			// this shouldn't ever happen
			return fmt.Errorf("failed to check rule existence: %v", err)
		}
		if exists {
			if _, ok := tablesRules[rule.table]; !ok {
				tablesRules[rule.table] = []IPTablesRestoreRuleSpec{}
			}
			tablesRules[rule.table] = append(tablesRules[rule.table], append(IPTablesRestoreRuleSpec{"-D", rule.chain}, rule.rulespec...))
		}
	}
	err := iptr.ApplyWithoutFlush(tablesRules) // ApplyWithoutFlush make a diff, Apply make a replace (desired state)
	if err != nil {
		return fmt.Errorf("unable to teardown iptables: %v", err)
	}

	return nil
}
