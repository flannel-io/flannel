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
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/flannel-io/flannel/pkg/trafficmngr"
	log "k8s.io/klog/v2"
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

type IPTablesManager struct{}

const kubeProxyMark string = "0x4000/0x4000"

func (iptm IPTablesManager) SetupAndEnsureMasqRules(flannelIPv4Net, prevSubnet ip.IP4Net,
	prevNetworks []ip.IP4Net,
	flannelIPv6Net, prevIPv6Subnet ip.IP6Net,
	prevIPv6Networks []ip.IP6Net,
	currentlease *lease.Lease,
	resyncPeriod int) error {
	if flannelIPv4Net.String() != "" {
		//Find the cidr in FLANNEL_NETWORK which contains the podCIDR (i.e. FLANNEL_SUBNET) of this node
		prevNetwork := ip.IP4Net{}
		for _, net := range prevNetworks {
			if net.ContainsCIDR(&prevSubnet) {
				prevNetwork = net
				break
			}
		}
		// recycle iptables rules only when network configured or subnet leased is not equal to current one.
		if prevNetwork != flannelIPv4Net && prevSubnet != currentlease.Subnet {
			log.Infof("Current network or subnet (%v, %v) is not equal to previous one (%v, %v), trying to recycle old iptables rules",
				flannelIPv4Net, currentlease.Subnet, prevNetwork, prevSubnet)
			newLease := &lease.Lease{
				Subnet: prevSubnet,
			}
			if err := iptm.deleteIP4Tables(iptm.masqRules(prevNetworks, newLease)); err != nil {
				return err
			}
		}

		log.Infof("Setting up masking rules")
		iptm.CreateIP4Chain("nat", "FLANNEL-POSTRTG")
		//Note: doesn't work for multiple networks but we disabled MultiClusterCIDR anyway
		getRules := func() []trafficmngr.IPTablesRule {
			return iptm.masqRules([]ip.IP4Net{flannelIPv4Net}, currentlease)
		}
		go iptm.setupAndEnsureIP4Tables(getRules, resyncPeriod)
	}
	if flannelIPv6Net.String() != "" {
		//Find the cidr in FLANNEL_IPV6_NETWORK which contains the podCIDR (i.e. FLANNEL_IPV6_SUBNET) of this node
		prevIPv6Network := ip.IP6Net{}
		for _, net := range prevIPv6Networks {
			if net.ContainsCIDR(&prevIPv6Subnet) {
				prevIPv6Network = net
				break
			}
		}
		// recycle iptables rules only when network configured or subnet leased is not equal to current one.
		if prevIPv6Network != flannelIPv6Net && prevIPv6Subnet != currentlease.IPv6Subnet {
			log.Infof("Current network or subnet (%v, %v) is not equal to previous one (%v, %v), trying to recycle old iptables rules",
				flannelIPv6Net, currentlease.IPv6Subnet, prevIPv6Network, prevIPv6Subnet)
			newLease := &lease.Lease{
				IPv6Subnet: prevIPv6Subnet,
			}
			if err := iptm.deleteIP6Tables(iptm.masqIP6Rules(prevIPv6Networks, newLease)); err != nil {
				return err
			}
		}

		log.Infof("Setting up masking rules for IPv6")
		iptm.CreateIP6Chain("nat", "FLANNEL-POSTRTG")
		//Note: doesn't work for multiple networks but we disabled MultiClusterCIDR anyway
		getRules := func() []trafficmngr.IPTablesRule {
			return iptm.masqIP6Rules([]ip.IP6Net{flannelIPv6Net}, currentlease)
		}
		go iptm.setupAndEnsureIP6Tables(getRules, resyncPeriod)
	}
	return nil
}

func (iptm IPTablesManager) masqRules(cluster_cidrs []ip.IP4Net, lease *lease.Lease) []trafficmngr.IPTablesRule {
	pod_cidr := lease.Subnet.String()
	ipt, err := iptables.New()
	supports_random_fully := false
	if err == nil {
		supports_random_fully = ipt.HasRandomFully()
	}
	rules := make([]trafficmngr.IPTablesRule, 2)
	// This rule ensure that the flannel iptables rules are executed before other rules on the node
	rules[0] = trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "POSTROUTING", Rulespec: []string{"-m", "comment", "--comment", "flanneld masq", "-j", "FLANNEL-POSTRTG"}}
	// This rule will not masquerade traffic marked by the kube-proxy to avoid double NAT bug on some kernel version
	rules[1] = trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-m", "mark", "--mark", kubeProxyMark, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}}
	for _, ccidr := range cluster_cidrs {
		cluster_cidr := ccidr.String()
		// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0), for any of the cluster_cidrs
		rules = append(rules,
			trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-s", pod_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-s", cluster_cidr, "-d", pod_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
		)
	}
	for _, ccidr := range cluster_cidrs {
		cluster_cidr := ccidr.String()
		// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
		rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"!", "-s", cluster_cidr, "-d", pod_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}})
	}
	for _, ccidr := range cluster_cidrs {
		cluster_cidr := ccidr.String()
		// NAT if it's not multicast traffic
		if supports_random_fully {
			rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-s", cluster_cidr, "!", "-d", "224.0.0.0/4", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}})
		} else {
			rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-s", cluster_cidr, "!", "-d", "224.0.0.0/4", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE"}})
		}
	}
	for _, ccidr := range cluster_cidrs {
		cluster_cidr := ccidr.String()
		// Masquerade anything headed towards flannel from the host
		if supports_random_fully {
			rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"!", "-s", cluster_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}})
		} else {
			rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"!", "-s", cluster_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE"}})
		}
	}
	return rules
}

func (iptm IPTablesManager) masqIP6Rules(cluster_cidrs []ip.IP6Net, lease *lease.Lease) []trafficmngr.IPTablesRule {
	pod_cidr := lease.IPv6Subnet.String()
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	supports_random_fully := false
	if err == nil {
		supports_random_fully = ipt.HasRandomFully()
	}
	rules := make([]trafficmngr.IPTablesRule, 2)

	// This rule ensure that the flannel iptables rules are executed before other rules on the node
	rules[0] = trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "POSTROUTING", Rulespec: []string{"-m", "comment", "--comment", "flanneld masq", "-j", "FLANNEL-POSTRTG"}}
	// This rule will not masquerade traffic marked by the kube-proxy to avoid double NAT bug on some kernel version
	rules[1] = trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-m", "mark", "--mark", kubeProxyMark, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}}

	for _, ccidr := range cluster_cidrs {
		cluster_cidr := ccidr.String()
		// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0), for any of the cluster_cidrs
		rules = append(rules,
			trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-s", pod_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
			trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-s", cluster_cidr, "-d", pod_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}},
		)
	}
	for _, ccidr := range cluster_cidrs {
		cluster_cidr := ccidr.String()
		// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
		rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"!", "-s", cluster_cidr, "-d", pod_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "RETURN"}})
	}
	for _, ccidr := range cluster_cidrs {
		cluster_cidr := ccidr.String()
		// NAT if it's not multicast traffic
		if supports_random_fully {
			rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-s", cluster_cidr, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}})
		} else {
			rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"-s", cluster_cidr, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE"}})
		}

	}
	for _, ccidr := range cluster_cidrs {
		cluster_cidr := ccidr.String()
		// Masquerade anything headed towards flannel from the host
		if supports_random_fully {
			rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"!", "-s", cluster_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE", "--random-fully"}})
		} else {
			rules = append(rules, trafficmngr.IPTablesRule{Table: "nat", Action: "-A", Chain: "FLANNEL-POSTRTG", Rulespec: []string{"!", "-s", cluster_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "flanneld masq", "-j", "MASQUERADE"}})
		}

	}

	return rules
}

func (iptm IPTablesManager) SetupAndEnsureForwardRules(flannelIPv4Network ip.IP4Net, flannelIPv6Network ip.IP6Net, resyncPeriod int) {
	if flannelIPv4Network.String() != "" {
		log.Infof("Changing default FORWARD chain policy to ACCEPT")
		iptm.CreateIP4Chain("filter", "FLANNEL-FWD")
		getRules := func() []trafficmngr.IPTablesRule {
			return iptm.forwardRules(flannelIPv4Network.String())
		}
		go iptm.setupAndEnsureIP4Tables(getRules, resyncPeriod)
	}
	if flannelIPv6Network.String() != "" {
		log.Infof("IPv6: Changing default FORWARD chain policy to ACCEPT")
		iptm.CreateIP6Chain("filter", "FLANNEL-FWD")
		getRules := func() []trafficmngr.IPTablesRule {
			return iptm.forwardRules(flannelIPv6Network.String())
		}
		go iptm.setupAndEnsureIP6Tables(getRules, resyncPeriod)
	}
}

func (iptm IPTablesManager) forwardRules(flannelNetwork string) []trafficmngr.IPTablesRule {
	return []trafficmngr.IPTablesRule{
		// This rule ensure that the flannel iptables rules are executed before other rules on the node
		{Table: "filter", Action: "-A", Chain: "FORWARD", Rulespec: []string{"-m", "comment", "--comment", "flanneld forward", "-j", "FLANNEL-FWD"}},
		// These rules allow traffic to be forwarded if it is to or from the flannel network range.
		{Table: "filter", Action: "-A", Chain: "FLANNEL-FWD", Rulespec: []string{"-s", flannelNetwork, "-m", "comment", "--comment", "flanneld forward", "-j", "ACCEPT"}},
		{Table: "filter", Action: "-A", Chain: "FLANNEL-FWD", Rulespec: []string{"-d", flannelNetwork, "-m", "comment", "--comment", "flanneld forward", "-j", "ACCEPT"}},
	}
}

func (iptm IPTablesManager) CreateIP4Chain(table, chain string) {
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

func (iptm IPTablesManager) CreateIP6Chain(table, chain string) {
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

func ipTablesRulesExist(ipt IPTables, rules []trafficmngr.IPTablesRule) (bool, error) {
	for _, rule := range rules {
		if rule.Chain == "FLANNEL-FWD" || rule.Rulespec[len(rule.Rulespec)-1] == "FLANNEL-FWD" {
			chainExist, err := ipt.ChainExists(rule.Table, "FLANNEL-FWD")
			if err != nil {
				return false, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				return false, nil
			}
		} else if rule.Chain == "FLANNEL-POSTRTG" || rule.Rulespec[len(rule.Rulespec)-1] == "FLANNEL-POSTRTG" {
			chainExist, err := ipt.ChainExists(rule.Table, "FLANNEL-POSTRTG")
			if err != nil {
				return false, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				return false, nil
			}
		}
		exists, err := ipt.Exists(rule.Table, rule.Chain, rule.Rulespec...)
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
func ipTablesCleanAndBuild(ipt IPTables, rules []trafficmngr.IPTablesRule) (IPTablesRestoreRules, error) {
	tablesRules := IPTablesRestoreRules{}

	// Build append and delete rules
	for _, rule := range rules {
		if rule.Chain == "FLANNEL-FWD" || rule.Rulespec[len(rule.Rulespec)-1] == "FLANNEL-FWD" {
			chainExist, err := ipt.ChainExists(rule.Table, "FLANNEL-FWD")
			if err != nil {
				return nil, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				err = ipt.ClearChain(rule.Table, "FLANNEL-FWD")
				if err != nil {
					return nil, fmt.Errorf("failed to create rule chain: %v", err)
				}
			}
		} else if rule.Chain == "FLANNEL-POSTRTG" || rule.Rulespec[len(rule.Rulespec)-1] == "FLANNEL-POSTRTG" {
			chainExist, err := ipt.ChainExists(rule.Table, "FLANNEL-POSTRTG")
			if err != nil {
				return nil, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				err = ipt.ClearChain(rule.Table, "FLANNEL-POSTRTG")
				if err != nil {
					return nil, fmt.Errorf("failed to create rule chain: %v", err)
				}
			}
		}
		exists, err := ipt.Exists(rule.Table, rule.Chain, rule.Rulespec...)
		if err != nil {
			// this shouldn't ever happen
			return nil, fmt.Errorf("failed to check rule existence: %v", err)
		}
		if exists {
			if _, ok := tablesRules[rule.Table]; !ok {
				tablesRules[rule.Table] = []IPTablesRestoreRuleSpec{}
			}
			// if the rule exists it's safer to delete it and then create them
			tablesRules[rule.Table] = append(tablesRules[rule.Table], append(IPTablesRestoreRuleSpec{"-D", rule.Chain}, rule.Rulespec...))
		}
		// with iptables-restore we can ensure that all rules created are in good order and have no external rule between them
		tablesRules[rule.Table] = append(tablesRules[rule.Table], append(IPTablesRestoreRuleSpec{rule.Action, rule.Chain}, rule.Rulespec...))
	}

	return tablesRules, nil
}

// ipTablesBootstrap init iptables rules using iptables-restore (with some cleaning if some rules already exists)
func ipTablesBootstrap(ipt IPTables, iptRestore IPTablesRestore, rules []trafficmngr.IPTablesRule) error {
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

func (iptm IPTablesManager) setupAndEnsureIP4Tables(getRules func() []trafficmngr.IPTablesRule, resyncPeriod int) {
	rules := getRules()
	log.Infof("generated %d rules", len(rules))
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
		if err := ensureIPTables(ipt, iptRestore, getRules()); err != nil {
			log.Errorf("Failed to ensure iptables rules: %v", err)
		}

		time.Sleep(time.Duration(resyncPeriod) * time.Second)
	}
}

func (iptm IPTablesManager) setupAndEnsureIP6Tables(getRules func() []trafficmngr.IPTablesRule, resyncPeriod int) {
	rules := getRules()
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
		if err := ensureIPTables(ipt, iptRestore, getRules()); err != nil {
			log.Errorf("Failed to ensure iptables rules: %v", err)
		}

		time.Sleep(time.Duration(resyncPeriod) * time.Second)
	}
}

// deleteIP4Tables delete specified iptables rules
func (iptm IPTablesManager) deleteIP4Tables(rules []trafficmngr.IPTablesRule) error {
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

// deleteIP6Tables delete specified iptables rules
func (iptm IPTablesManager) deleteIP6Tables(rules []trafficmngr.IPTablesRule) error {
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

func ensureIPTables(ipt IPTables, iptRestore IPTablesRestore, rules []trafficmngr.IPTablesRule) error {
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

func teardownIPTables(ipt IPTables, iptr IPTablesRestore, rules []trafficmngr.IPTablesRule) error {
	tablesRules := IPTablesRestoreRules{}

	// Build delete rules to a transaction for iptables restore
	for _, rule := range rules {
		if rule.Chain == "FLANNEL-FWD" || rule.Rulespec[len(rule.Rulespec)-1] == "FLANNEL-FWD" {
			chainExists, err := ipt.ChainExists(rule.Table, "FLANNEL-FWD")
			if err != nil {
				// this shouldn't ever happen
				return fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExists {
				continue
			}
		} else if rule.Chain == "FLANNEL-POSTRTG" || rule.Rulespec[len(rule.Rulespec)-1] == "FLANNEL-POSTRTG" {
			chainExists, err := ipt.ChainExists(rule.Table, "FLANNEL-POSTRTG")
			if err != nil {
				// this shouldn't ever happen
				return fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExists {
				continue
			}
		}
		exists, err := ipt.Exists(rule.Table, rule.Chain, rule.Rulespec...)
		if err != nil {
			// this shouldn't ever happen
			return fmt.Errorf("failed to check rule existence: %v", err)
		}
		if exists {
			if _, ok := tablesRules[rule.Table]; !ok {
				tablesRules[rule.Table] = []IPTablesRestoreRuleSpec{}
			}
			tablesRules[rule.Table] = append(tablesRules[rule.Table], append(IPTablesRestoreRuleSpec{"-D", rule.Chain}, rule.Rulespec...))
		}
	}
	err := iptr.ApplyWithoutFlush(tablesRules) // ApplyWithoutFlush make a diff, Apply make a replace (desired state)
	if err != nil {
		return fmt.Errorf("unable to teardown iptables: %v", err)
	}

	return nil
}
