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
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/flannel/pkg/ip"
)

type Registry interface {
	getNetworkConfig(ctx context.Context, network string) (string, error)
	getSubnets(ctx context.Context, network string) ([]Lease, uint64, error)
	getSubnet(ctx context.Context, network string, sn ip.IP4Net) (*Lease, uint64, error)
	createSubnet(ctx context.Context, network string, sn ip.IP4Net, attrs *LeaseAttrs, ttl time.Duration) (time.Time, error)
	updateSubnet(ctx context.Context, network string, sn ip.IP4Net, attrs *LeaseAttrs, ttl time.Duration, asof uint64) (time.Time, error)
	deleteSubnet(ctx context.Context, network string, sn ip.IP4Net) error
	watchSubnets(ctx context.Context, network string, since uint64) (Event, uint64, error)
	watchSubnet(ctx context.Context, network string, since uint64, sn ip.IP4Net) (Event, uint64, error)
	getNetworks(ctx context.Context) ([]string, uint64, error)
	watchNetworks(ctx context.Context, since uint64) (Event, uint64, error)
}

type EtcdConfig struct {
	Endpoints []string
	Keyfile   string
	Certfile  string
	CAFile    string
	Prefix    string
	Username  string
	Password  string
	UseV3API  bool
}
