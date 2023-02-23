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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	"golang.org/x/net/context"
)

var (
	ErrLeaseTaken  = errors.New("subnet: lease already taken")
	ErrNoMoreTries = errors.New("subnet: no more tries")
	subnetRegex    = regexp.MustCompile(`(\d+\.\d+.\d+.\d+)-(\d+)(?:&([a-f\d:]+)-(\d+))?$`)
)

type LeaseAttrs struct {
	PublicIP      ip.IP4
	PublicIPv6    *ip.IP6
	BackendType   string          `json:",omitempty"`
	BackendData   json.RawMessage `json:",omitempty"`
	BackendV6Data json.RawMessage `json:",omitempty"`
}

type Lease struct {
	EnableIPv4 bool
	EnableIPv6 bool
	Subnet     ip.IP4Net
	IPv6Subnet ip.IP6Net
	Attrs      LeaseAttrs
	Expiration time.Time

	Asof int64
}

func (l *Lease) Key() string {
	return MakeSubnetKey(l.Subnet, l.IPv6Subnet)
}

type (
	EventType int

	Event struct {
		Type  EventType `json:"type"`
		Lease Lease     `json:"lease,omitempty"`
	}
)

const (
	EventAdded EventType = iota
	EventRemoved
)

type LeaseWatchResult struct {
	// Either Events or Snapshot will be set.  If Events is empty, it means
	// the cursor was out of range and Snapshot contains the current list
	// of items, even if empty.
	Events   []Event     `json:"events"`
	Snapshot []Lease     `json:"snapshot"`
	Cursor   interface{} `json:"cursor"`
}

func (et EventType) MarshalJSON() ([]byte, error) {
	s := ""

	switch et {
	case EventAdded:
		s = "added"
	case EventRemoved:
		s = "removed"
	default:
		return nil, errors.New("bad event type")
	}
	return json.Marshal(s)
}

func (et *EventType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "\"added\"":
		*et = EventAdded
	case "\"removed\"":
		*et = EventRemoved
	default:
		fmt.Println(string(data))
		return errors.New("bad event type")
	}

	return nil
}

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
	AcquireLease(ctx context.Context, attrs *LeaseAttrs) (*Lease, error)
	RenewLease(ctx context.Context, lease *Lease) error
	WatchLease(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, receiver chan []LeaseWatchResult) error
	WatchLeases(ctx context.Context, receiver chan []LeaseWatchResult) error
	CompleteLease(ctx context.Context, lease *Lease, wg *sync.WaitGroup) error

	Name() string
}
