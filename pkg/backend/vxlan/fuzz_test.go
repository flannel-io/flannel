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

package vxlan

import (
	"encoding/json"
	"testing"
)

// FuzzParseVXLANConfig fuzzes parsing of the VXLAN backend configuration JSON,
// which originates from the untrusted network configuration.
func FuzzParseVXLANConfig(f *testing.F) {
	f.Add([]byte(`{"VNI":1,"Port":8472}`))
	f.Add([]byte(`{"MTU":1400,"GBP":true,"DirectRouting":true}`))
	f.Add([]byte(`{"Mac":"00:11:22:33:44:55"}`))
	f.Add([]byte(``))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseVXLANConfig(json.RawMessage(data), 1500)
	})
}
