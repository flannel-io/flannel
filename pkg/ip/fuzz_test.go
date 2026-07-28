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

package ip

import "testing"

// FuzzParseIP4 fuzzes parsing of IPv4 address strings.
func FuzzParseIP4(f *testing.F) {
	f.Add("10.0.0.1")
	f.Add("255.255.255.255")
	f.Add("0.0.0.0")
	f.Add("not-an-ip")
	f.Add("")

	f.Fuzz(func(t *testing.T, s string) {
		ip4, err := ParseIP4(s)
		if err != nil {
			return
		}
		// A successfully parsed address must round-trip back to a parseable
		// string representation.
		if _, err := ParseIP4(ip4.String()); err != nil {
			t.Fatalf("round-trip failed for %q -> %q: %v", s, ip4.String(), err)
		}
	})
}

// FuzzParseIP6 fuzzes parsing of IPv6 address strings.
func FuzzParseIP6(f *testing.F) {
	f.Add("fc00::1")
	f.Add("::1")
	f.Add("2001:db8::ff00:42:8329")
	f.Add("not-an-ip")
	f.Add("")

	f.Fuzz(func(t *testing.T, s string) {
		ip6, err := ParseIP6(s)
		if err != nil {
			return
		}
		if _, err := ParseIP6(ip6.String()); err != nil {
			t.Fatalf("round-trip failed for %q -> %q: %v", s, ip6.String(), err)
		}
	})
}

// FuzzIP4UnmarshalJSON fuzzes the json.Unmarshaler implementation for IP4.
func FuzzIP4UnmarshalJSON(f *testing.F) {
	f.Add([]byte(`"10.0.0.1"`))
	f.Add([]byte(`10.0.0.1`))
	f.Add([]byte(`""`))

	f.Fuzz(func(t *testing.T, j []byte) {
		var v IP4
		_ = v.UnmarshalJSON(j)
	})
}

// FuzzIP6UnmarshalJSON fuzzes the json.Unmarshaler implementation for IP6.
func FuzzIP6UnmarshalJSON(f *testing.F) {
	f.Add([]byte(`"fc00::1"`))
	f.Add([]byte(`fc00::1`))
	f.Add([]byte(`""`))

	f.Fuzz(func(t *testing.T, j []byte) {
		var v IP6
		_ = v.UnmarshalJSON(j)
	})
}

// FuzzIP4NetUnmarshalJSON fuzzes the json.Unmarshaler implementation for IP4Net.
func FuzzIP4NetUnmarshalJSON(f *testing.F) {
	f.Add([]byte(`"10.0.0.0/8"`))
	f.Add([]byte(`"10.0.0.0/33"`))
	f.Add([]byte(`""`))

	f.Fuzz(func(t *testing.T, j []byte) {
		var v IP4Net
		_ = v.UnmarshalJSON(j)
	})
}

// FuzzIP6NetUnmarshalJSON fuzzes the json.Unmarshaler implementation for IP6Net.
func FuzzIP6NetUnmarshalJSON(f *testing.F) {
	f.Add([]byte(`"fc00::/48"`))
	f.Add([]byte(`"fc00::/129"`))
	f.Add([]byte(`""`))

	f.Fuzz(func(t *testing.T, j []byte) {
		var v IP6Net
		_ = v.UnmarshalJSON(j)
	})
}
