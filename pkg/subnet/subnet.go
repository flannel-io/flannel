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

package subnet

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/google/renameio/v2"
	log "k8s.io/klog/v2"
)

var (
	subnetRegex = regexp.MustCompile(`(\d+\.\d+.\d+.\d+)-(\d+)(?:&([a-f\d:]+)-(\d+))?$`)
)

func ParseSubnetKey(s string) (*ip.IP4Net, *ip.IP6Net) {
	if parts := subnetRegex.FindStringSubmatch(s); len(parts) == 5 {
		snIp := net.ParseIP(parts[1]).To4()
		prefixLen, err := strconv.ParseUint(parts[2], 10, 5)

		if snIp == nil || err != nil {
			return nil, nil
		}
		sn4 := &ip.IP4Net{IP: ip.FromIP(snIp), PrefixLen: uint(prefixLen)}

		var sn6 *ip.IP6Net
		if parts[3] != "" {
			snIp6 := net.ParseIP(parts[3]).To16()
			prefixLen, err = strconv.ParseUint(parts[4], 10, 7)
			if snIp6 == nil || err != nil {
				return nil, nil
			}
			sn6 = &ip.IP6Net{IP: ip.FromIP6(snIp6), PrefixLen: uint(prefixLen)}
		}

		return sn4, sn6
	}

	return nil, nil
}

func MakeSubnetKey(sn ip.IP4Net, sn6 ip.IP6Net) string {
	if sn6.Empty() {
		return sn.StringSep(".", "-")
	} else {
		return sn.StringSep(".", "-") + "&" + sn6.StringSep(":", "-")
	}
}

func WriteSubnetFile(path string, config *Config, ipMasq bool, sn ip.IP4Net, ipv6sn ip.IP6Net, mtu int) error {
	dir, _ := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("mkdir subnet directory: %w", err)
	}

	perm := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	f, err := renameio.TempFile(dir, path)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer f.Cleanup()

	if err := f.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if config.EnableIPv4 {
		if _, err := fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", config.Network); err != nil {
			return fmt.Errorf("failed to write FLANNEL_NETWORK: %w", err)
		}
		sn.IncrementIP()
		if _, err := fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn); err != nil {
			return fmt.Errorf("failed to write FLANNEL_SUBNET: %w", err)
		}
	}
	if config.EnableIPv6 {
		if _, err := fmt.Fprintf(f, "FLANNEL_IPV6_NETWORK=%s\n", config.IPv6Network); err != nil {
			return fmt.Errorf("failed to write FLANNEL_IPV6_NETWORK: %w", err)
		}
		ipv6sn.IncrementIP()
		if _, err := fmt.Fprintf(f, "FLANNEL_IPV6_SUBNET=%s\n", ipv6sn); err != nil {
			return fmt.Errorf("failed to write FLANNEL_IPV6_SUBNET: %w", err)
		}
	}

	if _, err := fmt.Fprintf(f, "FLANNEL_MTU=%d\n", mtu); err != nil {
		return fmt.Errorf("failed to write FLANNEL_MTU: %w", err)
	}
	if _, err := fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq); err != nil {
		return fmt.Errorf("failed to write FLANNEL_IPMASQ: %w", err)
	}

	return f.CloseAtomicallyReplace()
}

type Manager interface {
	GetNetworkConfig(ctx context.Context) (*Config, error)
	HandleSubnetFile(path string, config *Config, ipMasq bool, sn ip.IP4Net, ipv6sn ip.IP6Net, mtu int) error
	AcquireLease(ctx context.Context, attrs *lease.LeaseAttrs) (*lease.Lease, error)
	RenewLease(ctx context.Context, lease *lease.Lease) error
	WatchLease(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, receiver chan []lease.LeaseWatchResult) error
	WatchLeases(ctx context.Context, receiver chan []lease.LeaseWatchResult) error
	CompleteLease(ctx context.Context, lease *lease.Lease, wg *sync.WaitGroup) error

	GetStoredMacAddresses(ctx context.Context) (string, string)
	GetStoredPublicIP(ctx context.Context) (string, string)
	Name() string
}

// WatchLeases performs a long term watch of the given network's subnet leases
// and communicates addition/deletion events on receiver channel. It takes care
// of handling "fall-behind" logic where the history window has advanced too far
// and it needs to diff the latest snapshot with its saved state and generate events
func WatchLeases(ctx context.Context, sm Manager, ownLease *lease.Lease, receiver chan []lease.Event) {

	// LeaseWatcher is initiated with the Lease of the local node
	lw := &lease.LeaseWatcher{
		OwnLease: ownLease,
	}

	leaseWatchChan := make(chan []lease.LeaseWatchResult)
	go func() {
		err := sm.WatchLeases(ctx, leaseWatchChan)
		if err != nil {
			log.Errorf("could not watch leases: %s", err)
			return
		}
	}()
	for watchResults := range leaseWatchChan {
		for _, wr := range watchResults {
			var batch []lease.Event

			if len(wr.Events) > 0 {
				batch = lw.Update(wr.Events)
			} else {
				batch = lw.Reset(wr.Snapshot)
			}

			for i := range batch {
				log.Infof("Batch elem [%d] is { %#v }", i, batch[i])
			}
			if len(batch) > 0 {
				receiver <- batch
			}
		}
	}

	close(receiver)
}

// WatchLease performs a long term watch of the given network's subnet lease
// and communicates addition/deletion events on receiver channel. It takes care
// of handling "fall-behind" logic where the history window has advanced too far
// and it needs to diff the latest snapshot with its saved state and generate events
func WatchLease(ctx context.Context, sm Manager, sn ip.IP4Net, sn6 ip.IP6Net, receiver chan lease.Event) {
	leaseWatchChan := make(chan []lease.LeaseWatchResult)
	var closeOnce sync.Once

	// Helper function to close the receiver channel safely
	closeReceiver := func() {
		closeOnce.Do(func() {
			close(receiver)
		})
	}

	go func() {
		err := sm.WatchLease(ctx, sn, sn6, leaseWatchChan)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				log.Infof("%v, close receiver chan", err)
				closeReceiver()
				return
			}

			log.Errorf("Subnet watch failed: %v", err)
			closeReceiver()
			return
		}

	}()

	for watchResults := range leaseWatchChan {
		for _, wr := range watchResults {
			if len(wr.Snapshot) > 0 {
				receiver <- lease.Event{
					Type:  lease.EventAdded,
					Lease: wr.Snapshot[0],
				}
			} else if len(wr.Events) > 0 {
				receiver <- wr.Events[0]
			} else {
				log.V(2).Info("WatchLease: empty event received")
			}
		}

	}
	log.Info("leaseWatchChan channel closed")
	closeReceiver()
}
