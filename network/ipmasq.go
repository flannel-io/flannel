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
	"fmt"
	"strings"

	log "github.com/golang/glog"

	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

type IPTablesRules interface {
	AppendUnique(table string, chain string, rulespec ...string) error
	Delete(table string, chain string, rulespec ...string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
}

func rules(ipn ip.IP4Net, lease *subnet.Lease) [][]string {
	n := ipn.String()
	sn := lease.Subnet.String()

	return [][]string{
		// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
		{"-s", n, "-d", n, "-j", "RETURN"},
		// NAT if it's not multicast traffic
		{"-s", n, "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE"},
		// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
		{"!", "-s", n, "-d", sn, "-j", "RETURN"},
		// Masquerade anything headed towards flannel from the host
		{"!", "-s", n, "-d", n, "-j", "MASQUERADE"},
	}
}

func ipMasqRulesExist(ipt IPTablesRules, ipn ip.IP4Net, lease *subnet.Lease) (bool, error) {
	for _, rule := range rules(ipn, lease) {
		exists, err := ipt.Exists("nat", "POSTROUTING", rule...)
		if err != nil {
			// this shouldn't ever happen
			return false, fmt.Errorf("failed to check rule existence", err)
		}
		if !exists {
			return false, nil
		}
	}

	return true, nil
}

func EnsureIPMasq(ipt IPTablesRules, ipn ip.IP4Net, lease *subnet.Lease) error {
	exists, err := ipMasqRulesExist(ipt, ipn, lease)
	if err != nil {
		return fmt.Errorf("Error checking rule existence: %v", err)
	}
	if exists {
		// if all the rules already exist, no need to do anything
		return nil
	}
	// Otherwise, teardown all the rules and set them up again
	// We do this because the order of the rules is important
	log.Info("Some iptables rules are missing; deleting and recreating rules")
	TeardownIPMasq(ipt, ipn, lease)
	if err = SetupIPMasq(ipt, ipn, lease); err != nil {
		return fmt.Errorf("Error setting up rules: %v", err)
	}
	return nil
}

func SetupIPMasq(ipt IPTablesRules, ipn ip.IP4Net, lease *subnet.Lease) error {
	for _, rule := range rules(ipn, lease) {
		log.Info("Adding iptables rule: ", strings.Join(rule, " "))
		err := ipt.AppendUnique("nat", "POSTROUTING", rule...)
		if err != nil {
			return fmt.Errorf("failed to insert IP masquerade rule: %v", err)
		}
	}

	return nil
}

func TeardownIPMasq(ipt IPTablesRules, ipn ip.IP4Net, lease *subnet.Lease) {
	for _, rule := range rules(ipn, lease) {
		log.Info("Deleting iptables rule: ", strings.Join(rule, " "))
		// We ignore errors here because if there's an error it's almost certainly because the rule
		// doesn't exist, which is fine (we don't need to delete rules that don't exist)
		ipt.Delete("nat", "POSTROUTING", rule...)
	}
}
