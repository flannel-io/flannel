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
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

var CharonPath string

const (
	defaultCharonPath = "/opt/flannel/libexec/ipsec/charon"
	passwordLength    = 40
)

func init() {
	backend.Register("ipsec", New)
}

type IPSECBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := &IPSECBackend{
		sm:       sm,
		extIface: extIface,
	}

	return be, nil
}

func (be *IPSECBackend) RegisterNetwork(ctx context.Context, netname string, config *subnet.Config) (backend.Network, error) {
	cfg := struct {
		UDPEncap bool
	}{}

	if len(config.Backend) > 0 {
		log.Info("i.config.backend length > 0")
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding IPSEC backend config: %v", err)
		}
	}

	attrs := subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(be.extIface.ExtAddr),
		BackendType: "ipsec",
	}

	l, err := be.sm.AcquireLease(ctx, netname, &attrs)

	switch err {
	case nil:

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	if CharonPath == "" {
		CharonPath = defaultCharonPath
	}

	ikeDaemon, err := NewCharonIKEDaemon(CharonPath)
	if err != nil {
		return nil, fmt.Errorf("error creating CharonIKEDaemon struct: %v", err)
	}

	log.Info("UDPEncap: ", cfg.UDPEncap)

	password, err := GenerateRandomString(passwordLength)
	if err != nil {
		return nil, fmt.Errorf("error generating random string: %v", err)
	}

	err = be.sm.CreateBackendData(ctx, netname, password)
	if err != nil {
		return nil, fmt.Errorf("error creating password: %v", err)
	}

	password, err = be.sm.GetBackendData(ctx, netname)
	if err != nil {
		return nil, fmt.Errorf("error getting password: %v", err)
	}

	return newNetwork(netname, be.sm, be.extIface, cfg.UDPEncap, password, ikeDaemon, l)
}

func (be *IPSECBackend) Run(ctx context.Context) {
	<-ctx.Done()
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)

	if err != nil {
		return nil, err
	}

	return b, nil
}

func GenerateRandomString(s int) (string, error) {
	b, err := generateRandomBytes(s)

	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}
