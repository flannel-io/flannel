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

package config

import (
	"encoding/json"
	"fmt"

	"github.com/flannel-io/flannel/pkg/config/etcd"
	"github.com/flannel-io/flannel/pkg/config/kube"
)

type SubnetManagerType uint

const (
	EtcdType SubnetManagerType = iota
	KubeType
)

type Config struct {
	etcd.EtcdSubnetMngrConfig
	kube.KubeSubnetMngrConfig
	EnableIPv4  bool
	EnableIPv6  bool
	BackendType string          `json:"-"`
	Backend     json.RawMessage `json:",omitempty"`
}

func parseBackendType(be json.RawMessage) (string, error) {
	var bt struct {
		Type string
	}

	if len(be) == 0 {
		return "udp", nil
	} else if err := json.Unmarshal(be, &bt); err != nil {
		return "", fmt.Errorf("error decoding Backend property of config: %v", err)
	}

	return bt.Type, nil
}

func ParseConfig(s string, t SubnetManagerType) (*Config, error) {
	cfg := new(Config)
	// Enable ipv4 by default
	cfg.EnableIPv4 = true
	err := json.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}

	if t == EtcdType {
		err = etcd.CheckConfig(&cfg.EtcdSubnetMngrConfig, cfg.EnableIPv4, cfg.EnableIPv6)
	} else if t == KubeType {
		err = kube.CheckConfig(&cfg.KubeSubnetMngrConfig, cfg.EnableIPv4, cfg.EnableIPv6)
	} else {
		return nil, fmt.Errorf("unknown SubnetManagerType: %d", t)
	}

	if err != nil {
		return nil, err
	}

	bt, err := parseBackendType(cfg.Backend)
	if err != nil {
		return nil, err
	}
	cfg.BackendType = bt

	return cfg, nil
}
