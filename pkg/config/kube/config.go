// Copyright 2022 flannel authors
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

package kube

import "github.com/flannel-io/flannel/pkg/ip"

type KubeSubnetMngrConfig struct {
	Networks     []ip.IP4Net
	IPv6Networks []ip.IP6Net
}

func CheckConfig(ksmConfig *KubeSubnetMngrConfig, enableIPv4, enableIPv6 bool) error {
	return nil
}
