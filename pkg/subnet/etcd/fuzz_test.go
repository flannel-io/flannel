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

package etcd

import (
	"testing"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

// FuzzKvToIPLease fuzzes the conversion of an etcd key/value pair into a flannel
// lease. Both the key (a subnet key such as "10.5.1.0-24") and the value (the
// LeaseAttrs JSON, which includes the custom-unmarshalled PublicIP/PublicIPv6
// fields) are read straight out of etcd and are therefore untrusted. kvToIPLease
// must never panic on malformed data.
func FuzzKvToIPLease(f *testing.F) {
	f.Add([]byte("10.5.1.0-24"), []byte(`{"PublicIP":"192.168.0.1","BackendType":"vxlan"}`), int64(3600))
	f.Add([]byte("10.5.1.0-24&fc00::-64"), []byte(`{"PublicIP":"192.168.0.1","PublicIPv6":"fc00::1"}`), int64(60))
	f.Add([]byte("10.5.1.0-24"), []byte(`{"PublicIP":"2001:db8::1"}`), int64(0))
	f.Add([]byte("not-a-subnet"), []byte(`{}`), int64(-1))
	f.Add([]byte(""), []byte(""), int64(0))

	f.Fuzz(func(t *testing.T, key, value []byte, ttl int64) {
		kv := &mvccpb.KeyValue{
			Key:   key,
			Value: value,
		}
		// Only require that kvToIPLease does not panic on arbitrary input;
		// an error for malformed data is expected behaviour.
		_, _ = kvToIPLease(kv, ttl)
	})
}
