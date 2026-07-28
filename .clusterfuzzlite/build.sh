#!/bin/bash -eu
# Copyright 2024 flannel authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# build.sh is invoked by the OSS-Fuzz / ClusterFuzzLite base-builder-go image.
# It compiles every native Go fuzz target (func FuzzXxx(f *testing.F)) into a
# libFuzzer binary placed in $OUT.
#
# Arguments to compile_native_go_fuzzer:
#   <go package> <FuzzFunc name> <output binary name>

cd "$SRC/flannel"

# flannel only builds on linux; keep the toolchain from trying anything else.
export GOOS=linux
export CGO_ENABLED=1

# compile_native_go_fuzzer generates a wrapper that imports this helper package.
go get github.com/AdamKorcz/go-118-fuzz-build/testing@v0.0.0-20250520111509-a70c2aa677fa

# package_path::FuzzFuncName::output_name
targets=(
  "./pkg/subnet::FuzzParseConfig::fuzz_parse_config"
  "./pkg/subnet::FuzzParseSubnetKey::fuzz_parse_subnet_key"
  "./pkg/subnet/kube::FuzzNodeToLease::fuzz_node_to_lease"
  "./pkg/ip::FuzzParseIP4::fuzz_parse_ip4"
  "./pkg/ip::FuzzParseIP6::fuzz_parse_ip6"
  "./pkg/ip::FuzzIP4UnmarshalJSON::fuzz_ip4_unmarshal_json"
  "./pkg/ip::FuzzIP6UnmarshalJSON::fuzz_ip6_unmarshal_json"
  "./pkg/ip::FuzzIP4NetUnmarshalJSON::fuzz_ip4net_unmarshal_json"
  "./pkg/ip::FuzzIP6NetUnmarshalJSON::fuzz_ip6net_unmarshal_json"
  "./pkg/backend/vxlan::FuzzParseVXLANConfig::fuzz_parse_vxlan_config"
)

for target in "${targets[@]}"; do
  pkg="${target%%::*}"
  rest="${target#*::}"
  func="${rest%%::*}"
  out="${rest#*::}"
  echo "Building fuzzer ${out} (${func} in ${pkg})"
  compile_native_go_fuzzer "${pkg}" "${func}" "${out}"
done
