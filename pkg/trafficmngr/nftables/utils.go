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

	log "k8s.io/klog/v2"
	"sigs.k8s.io/knftables"
)

const (
	masqueradeTestTable = "masqueradeTest"
)

// check whether masquerade fully-random is supported by the kernel
func (nftm *NFTablesManager) checkRandomfully(ctx context.Context) bool {
	result := true
	tx := nftm.nftv4.NewTransaction()
	tx.Add(&knftables.Chain{
		Name:     masqueradeTestTable,
		Comment:  knftables.PtrTo("chain to test if masquerade random fully is supported"),
		Type:     knftables.PtrTo(knftables.NATType),
		Hook:     knftables.PtrTo(knftables.PostroutingHook),
		Priority: knftables.PtrTo(knftables.SNATPriority),
	})
	tx.Flush(&knftables.Chain{
		Name: masqueradeTestTable,
	})
	// Masquerade anything headed towards flannel from the host
	tx.Add(&knftables.Rule{
		Chain: masqueradeTestTable,
		Rule: knftables.Concat(
			"ip saddr", "!=", "127.0.0.1",
			"masquerade fully-random",
		),
	})
	err := nftm.nftv4.Check(ctx, tx)
	if err != nil {
		log.Warningf("nftables: random fully unsupported")
		result = false
	}
	return result
}
