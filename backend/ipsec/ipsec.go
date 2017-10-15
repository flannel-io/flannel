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

package ipsec

import (
	"encoding/json"
	"fmt"
	log "github.com/golang/glog"
	"golang.org/x/net/context"
	"sync"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

var CharonViciUri string

const (
	defaultESPProposal = "aes128gcm16-sha256-prfsha256-ecp256"
	minPasswordLength  = 96
)

func init() {
	backend.Register("ipsec", New)
}

type IPSECBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (
	backend.Backend, error) {
	be := &IPSECBackend{
		sm:       sm,
		extIface: extIface,
	}

	return be, nil
}

func (be *IPSECBackend) RegisterNetwork(
	ctx context.Context, wg sync.WaitGroup, config *subnet.Config) (backend.Network, error) {

	cfg := struct {
		UDPEncap    bool
		ESPProposal string
		PSK         string
	}{
		UDPEncap:    false,
		ESPProposal: defaultESPProposal,
	}

	if len(config.Backend) > 0 {
		log.Info("i.config.backend length > 0")
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding IPSEC backend config: %v", err)
		}
	}

	if len(cfg.PSK) < minPasswordLength {
		return nil, fmt.Errorf(
			"config error, password should be at least %s characters long",
			minPasswordLength)
	}

	attrs := subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(be.extIface.ExtAddr),
		BackendType: "ipsec",
	}

	l, err := be.sm.AcquireLease(ctx, &attrs)

	switch err {
	case nil:

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	ikeDaemon, err := NewCharonIKEDaemon(ctx, wg, CharonViciUri, cfg.ESPProposal)
	if err != nil {
		return nil, fmt.Errorf("error creating CharonIKEDaemon struct: %v", err)
	}

	log.Info("UDPEncap: ", cfg.UDPEncap)

	return newNetwork(be.sm, be.extIface, cfg.UDPEncap, cfg.PSK, ikeDaemon, l)
}

func (be *IPSECBackend) Run(ctx context.Context) {
	<-ctx.Done()
}
