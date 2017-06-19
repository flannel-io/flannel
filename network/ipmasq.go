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

	"github.com/coreos/go-iptables/iptables"
	log "github.com/golang/glog"

	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

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

func SetupIPMasq(ipn ip.IP4Net, lease *subnet.Lease) error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to set up IP Masquerade. iptables was not found")
	}

	for _, rule := range rules(ipn, lease) {
		log.Info("Adding iptables rule: ", strings.Join(rule, " "))
		err = ipt.AppendUnique("nat", "POSTROUTING", rule...)
		if err != nil {
			return fmt.Errorf("failed to insert IP masquerade rule: %v", err)
		}
	}

	return nil
}

func TeardownIPMasq(ipn ip.IP4Net, lease *subnet.Lease) error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to teardown IP Masquerade. iptables was not found")
	}

	for _, rule := range rules(ipn, lease) {
		log.Info("Deleting iptables rule: ", strings.Join(rule, " "))
		err = ipt.Delete("nat", "POSTROUTING", rule...)
		if err != nil {
			return fmt.Errorf("failed to delete IP masquerade rule: %v", err)
		}
	}

	return nil
}
