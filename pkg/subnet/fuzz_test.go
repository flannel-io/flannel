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

package subnet

import "testing"

// FuzzParseConfig fuzzes the parsing of the flannel network configuration JSON.
// ParseConfig consumes untrusted input (the network config coming from etcd or
// the Kubernetes API), so it must never panic on malformed data.
func FuzzParseConfig(f *testing.F) {
	f.Add(`{"Network":"10.0.0.0/8"}`)
	f.Add(`{"Network":"10.0.0.0/8","SubnetLen":24,"Backend":{"Type":"vxlan"}}`)
	f.Add(`{"EnableIPv6":true,"IPv6Network":"fc00::/48","Backend":{"Type":"host-gw"}}`)
	f.Add(`{"Network":"10.0.0.0/8","SubnetMin":"10.0.1.0","SubnetMax":"10.0.20.0"}`)
	f.Add(``)
	f.Add(`{}`)

	f.Fuzz(func(t *testing.T, s string) {
		// We only care that ParseConfig does not panic or hang on arbitrary
		// input. A returned error for malformed input is expected behaviour.
		_, _ = ParseConfig(s)
	})
}

// FuzzParseSubnetKey fuzzes the parsing of subnet keys such as "10.5.1.0-24"
// or "10.5.1.0-24&fc00::-64", which are read back from the datastore.
func FuzzParseSubnetKey(f *testing.F) {
	f.Add("10.5.1.0-24")
	f.Add("10.5.1.0-24&fc00::-64")
	f.Add("255.255.255.255-32")
	f.Add("garbage")
	f.Add("")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseSubnetKey(s)
	})
}
