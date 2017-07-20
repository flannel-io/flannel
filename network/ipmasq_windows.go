// +build windows

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
	"errors"

	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

func SetupIPMasq(ipn ip.IP4Net, lease *subnet.Lease) error {
	// TODO: ignore for now, this is used by the ipmasq option to setup a POSTROUTING nat rule
	// to use to reach addresses outside the flannel network
	return errors.New("SetupIPMasq not implemented for this platform")
}

func TeardownIPMasq(ipn ip.IP4Net, lease *subnet.Lease) error {
	return errors.New("TeardownIPMasq not implemented for this platform")
}
