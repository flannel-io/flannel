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
//go:build !windows
// +build !windows

package nftables

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "k8s.io/klog/v2"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"sigs.k8s.io/knftables"
)

const (
	ipv4Table    = "flannel-ipv4"
	ipv6Table    = "flannel-ipv6"
	forwardChain = "forward"
	postrtgChain = "postrtg"
	//maximum delay in second to clean-up when the context is cancelled
	cleanUpDeadline = 15
)

type NFTablesManager struct {
	nftv4 knftables.Interface
	nftv6 knftables.Interface
}

func (nftm *NFTablesManager) Init(ctx context.Context, wg *sync.WaitGroup) error {
	log.Info("Starting flannel in nftables mode...")
	var err error
	nftm.nftv4, err = initTable(ctx, knftables.IPv4Family, ipv4Table)
	if err != nil {
		return err
	}
	nftm.nftv6, err = initTable(ctx, knftables.IPv6Family, ipv6Table)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		<-ctx.Done()
		log.Info("Cleaning-up flannel tables...")

		cleanupCtx, cleanUpCancelFunc := context.WithTimeout(context.Background(), cleanUpDeadline*time.Second)
		defer cleanUpCancelFunc()
		err := nftm.cleanUp(cleanupCtx)
		log.Errorf("nftables: error while cleaning-up: %v", err)
		wg.Done()
	}()
	return nil
}

// create a new table and returns the interface to interact with it
func initTable(ctx context.Context, ipFamily knftables.Family, name string) (knftables.Interface, error) {
	nft, err := knftables.New(ipFamily, name)
	if err != nil {
		return nil, fmt.Errorf("no nftables support: %v", err)
	}
	tx := nft.NewTransaction()

	tx.Add(&knftables.Table{
		Comment: knftables.PtrTo("rules for " + name),
	})
	err = nft.Run(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("nftables: couldn't initialise table %s: %v", name, err)
	}
	return nft, nil
}

// It is needed when using nftables? accept seems to be the default
// warning: never add a default 'drop' policy on the forwardChain as it breaks connectivity to the node
func (nftm *NFTablesManager) SetupAndEnsureForwardRules(ctx context.Context,
	flannelIPv4Network ip.IP4Net, flannelIPv6Network ip.IP6Net, resyncPeriod int) {
	if !flannelIPv4Network.Empty() {
		log.Infof("Changing default FORWARD chain policy to ACCEPT")
		tx := nftm.nftv4.NewTransaction()

		tx.Add(&knftables.Chain{
			Name:     forwardChain,
			Comment:  knftables.PtrTo("chain to accept flannel traffic"),
			Type:     knftables.PtrTo(knftables.FilterType),
			Hook:     knftables.PtrTo(knftables.ForwardHook),
			Priority: knftables.PtrTo(knftables.FilterPriority),
		})
		tx.Flush(&knftables.Chain{
			Name: forwardChain,
		})

		tx.Add(&knftables.Rule{
			Chain: forwardChain,
			Rule: knftables.Concat(
				"ip saddr", flannelIPv4Network.String(),
				"accept",
			),
		})
		tx.Add(&knftables.Rule{
			Chain: forwardChain,
			Rule: knftables.Concat(
				"ip daddr", flannelIPv4Network.String(),
				"accept",
			),
		})
		err := nftm.nftv4.Run(ctx, tx)
		if err != nil {
			log.Errorf("nftables: couldn't setup forward rules: %v", err)
		}
	}
	if !flannelIPv6Network.Empty() {
		log.Infof("Changing default FORWARD chain policy to ACCEPT (ipv6)")
		tx := nftm.nftv6.NewTransaction()

		tx.Add(&knftables.Chain{
			Name:     forwardChain,
			Comment:  knftables.PtrTo("chain to accept flannel traffic"),
			Type:     knftables.PtrTo(knftables.FilterType),
			Hook:     knftables.PtrTo(knftables.ForwardHook),
			Priority: knftables.PtrTo(knftables.FilterPriority),
		})
		tx.Flush(&knftables.Chain{
			Name: forwardChain,
		})

		tx.Add(&knftables.Rule{
			Chain: forwardChain,
			Rule: knftables.Concat(
				"ip6 saddr", flannelIPv6Network.String(),
				"accept",
			),
		})
		tx.Add(&knftables.Rule{
			Chain: forwardChain,
			Rule: knftables.Concat(
				"ip6 daddr", flannelIPv6Network.String(),
				"accept",
			),
		})
		err := nftm.nftv6.Run(ctx, tx)
		if err != nil {
			log.Errorf("nftables: couldn't setup forward rules (ipv6): %v", err)
		}
	}
}

func (nftm *NFTablesManager) SetupAndEnsureMasqRules(ctx context.Context, flannelIPv4Net, prevSubnet, prevNetwork ip.IP4Net,
	flannelIPv6Net, prevIPv6Subnet, prevIPv6Network ip.IP6Net,
	currentlease *lease.Lease,
	resyncPeriod int) error {
	if !flannelIPv4Net.Empty() {
		log.Infof("nftables: setting up masking rules (ipv4)")
		tx := nftm.nftv4.NewTransaction()

		tx.Add(&knftables.Chain{
			Name:     postrtgChain,
			Comment:  knftables.PtrTo("chain to manage traffic masquerading by flannel"),
			Type:     knftables.PtrTo(knftables.NATType),
			Hook:     knftables.PtrTo(knftables.PostroutingHook),
			Priority: knftables.PtrTo(knftables.SNATPriority),
		})
		// make sure that the chain is empty before adding our rules
		// => no need for the check and recycle part of iptables.go
		tx.Flush(&knftables.Chain{
			Name: postrtgChain,
		})
		err := nftm.addMasqRules(ctx, tx, flannelIPv4Net.String(), currentlease.Subnet.String(), knftables.IPv4Family)
		if err != nil {
			return fmt.Errorf("nftables: couldn't setup masq rules: %v", err)
		}
		err = nftm.nftv4.Run(ctx, tx)
		if err != nil {
			return fmt.Errorf("nftables: couldn't setup masq rules: %v", err)
		}
	}
	if !flannelIPv6Net.Empty() {
		log.Infof("nftables: setting up masking rules (ipv6)")
		tx := nftm.nftv6.NewTransaction()

		tx.Add(&knftables.Chain{
			Name:     postrtgChain,
			Comment:  knftables.PtrTo("chain to manage traffic masquerading by flannel"),
			Type:     knftables.PtrTo(knftables.NATType),
			Hook:     knftables.PtrTo(knftables.PostroutingHook),
			Priority: knftables.PtrTo(knftables.SNATPriority),
		})
		// make sure that the chain is empty before adding our rules
		// => no need for the check and recycle part of iptables.go
		tx.Flush(&knftables.Chain{
			Name: postrtgChain,
		})
		err := nftm.addMasqRules(ctx, tx, flannelIPv6Net.String(), currentlease.IPv6Subnet.String(), knftables.IPv6Family)
		if err != nil {
			return fmt.Errorf("nftables: couldn't setup masq rules: %v", err)
		}
		err = nftm.nftv6.Run(ctx, tx)
		if err != nil {
			return fmt.Errorf("nftables: couldn't setup masq rules: %v", err)
		}
	}
	return nil
}

// add required masking rules to transaction tx
func (nftm *NFTablesManager) addMasqRules(ctx context.Context,
	tx *knftables.Transaction,
	clusterCidr, podCidr string,
	family knftables.Family) error {
	masquerade := "masquerade fully-random"
	if !nftm.checkRandomfully(ctx) {
		masquerade = "masquerade"
	}

	multicastCidr := "224.0.0.0/4"
	if family == knftables.IPv6Family {
		multicastCidr = "ff00::/8"
	}
	// This rule will not masquerade traffic marked
	// by the kube-proxy to avoid double NAT bug on some kernel version
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			"meta mark", "0x4000", //TODO_TF: check the correct value when deploying kube-proxy
			"return",
		),
	})
	// don't NAT traffic within overlay network
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", podCidr,
			family, "daddr", clusterCidr,
			"return",
		),
	})
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", clusterCidr,
			family, "daddr", podCidr,
			"return",
		),
	})
	// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", "!=", podCidr,
			family, "daddr", clusterCidr,
			"return",
		),
	})
	// NAT if it's not multicast traffic
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", clusterCidr,
			family, "daddr", "!=", multicastCidr,
			masquerade,
		),
	})
	// Masquerade anything headed towards flannel from the host
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", "!=", clusterCidr,
			family, "daddr", clusterCidr,
			masquerade,
		),
	})
	return nil
}

// clean-up all nftables states created by flannel by deleting all related tables
func (nftm *NFTablesManager) cleanUp(ctx context.Context) error {
	nft, err := knftables.New(knftables.IPv4Family, ipv4Table)
	if err == nil {
		tx := nft.NewTransaction()
		tx.Delete(&knftables.Table{})
		err = nft.Run(ctx, tx)
	}
	if err != nil {
		return fmt.Errorf("nftables: couldn't delete table: %v", err)
	}

	nft, err = knftables.New(knftables.IPv6Family, ipv6Table)
	if err == nil {
		tx := nft.NewTransaction()
		tx.Delete(&knftables.Table{})
		err = nft.Run(ctx, tx)
	}
	if err != nil {
		return fmt.Errorf("nftables (ipv6): couldn't delete table: %v", err)
	}

	return nil
}
