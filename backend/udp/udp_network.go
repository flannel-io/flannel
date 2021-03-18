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
// +build !amd64,!windows

package udp

import (
	"fmt"

	"github.com/flannel-io/flannel/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/subnet"
)

func newNetwork(sm subnet.Manager, extIface *backend.ExternalInterface, port int, nw ip.IP4Net, l *subnet.Lease) (*backend.SimpleNetwork, error) {
	return nil, fmt.Errorf("UDP backend is not supported on this architecture")
}
