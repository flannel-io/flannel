package main

import (
	"fmt"
	"strings"

	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/flannel/pkg/ip"
)

func setupIPMasq(ipn ip.IP4Net) error {
	ipt, err := ip.NewIPTables()
	if err != nil {
		return fmt.Errorf("failed to setup IP Masquerade. iptables was not found")
	}

	err = ipt.ClearChain("nat", "FLANNEL")
	if err != nil {
		return fmt.Errorf("Failed to create/clear FLANNEL chain in NAT table: %v", err)
	}

	rules := [][]string{
		// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
		{"FLANNEL", "-d", ipn.String(), "-j", "ACCEPT"},
		// NAT if it's not multicast traffic
		{"FLANNEL", "!", "-d", "224.0.0.0/4", "-j", "MASQUERADE"},
		// This rule will take everything coming from overlay and sent it to FLANNEL chain
		{"POSTROUTING", "-s", ipn.String(), "-j", "FLANNEL"},
	}

	for _, args := range rules {
		log.Info("Adding iptables rule: ", strings.Join(args, " "))

		err = ipt.AppendUnique("nat", args...)
		if err != nil {
			return fmt.Errorf("Failed to insert IP masquerade rule: %v", err)
		}
	}

	return nil
}
