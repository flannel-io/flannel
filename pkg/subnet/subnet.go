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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"golang.org/x/net/context"
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
	dir, name := filepath.Split(path)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	tempFile := filepath.Join(dir, "."+name)
	f, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	if config.EnableIPv4 {
		if config.HasNetworks() {
			fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", strings.Join(ip.MapIP4ToString(config.Networks), ","))
		} else {
			fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", config.Network)
		}
		// Write out the first usable IP by incrementing sn.IP by one
		sn.IncrementIP()

		fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn)
	}
	if config.EnableIPv6 {
		if config.HasIPv6Networks() {
			fmt.Fprintf(f, "FLANNEL_IPV6_NETWORK=%s\n", strings.Join(ip.MapIP6ToString(config.IPv6Networks), ","))
		} else {
			fmt.Fprintf(f, "FLANNEL_IPV6_NETWORK=%s\n", config.IPv6Network)
		}
		// Write out the first usable IP by incrementing ip6Sn.IP by one
		ipv6sn.IncrementIP()
		fmt.Fprintf(f, "FLANNEL_IPV6_SUBNET=%s\n", ipv6sn)
	}

	fmt.Fprintf(f, "FLANNEL_MTU=%d\n", mtu)
	_, err = fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq)
	f.Close()
	if err != nil {
		return err
	}

	// rename(2) the temporary file to the desired location so that it becomes
	// atomically visible with the contents
	return os.Rename(tempFile, path)
	// TODO - is this safe? What if it's not on the same FS?
}

type Manager interface {
	GetNetworkConfig(ctx context.Context) (*Config, error)
	HandleSubnetFile(path string, config *Config, ipMasq bool, sn ip.IP4Net, ipv6sn ip.IP6Net, mtu int) error
	AcquireLease(ctx context.Context, attrs *lease.LeaseAttrs) (*lease.Lease, error)
	RenewLease(ctx context.Context, lease *lease.Lease) error
	WatchLease(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, receiver chan []lease.LeaseWatchResult) error
	WatchLeases(ctx context.Context, receiver chan []lease.LeaseWatchResult) error
	CompleteLease(ctx context.Context, lease *lease.Lease, wg *sync.WaitGroup) error

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

	go func() {
		err := sm.WatchLease(ctx, sn, sn6, leaseWatchChan)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				log.Infof("%v, close receiver chan", err)
				close(receiver)
				return
			}

			log.Errorf("Subnet watch failed: %v", err)
			close(receiver)
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
	close(receiver)
}
