// Copyright 2017 flannel authors
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

package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/flannel-io/flannel/pkg/subnet"
	log "k8s.io/klog/v2"
)

func init() {
	backend.Register("extension", New)
}

type ExtensionBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
	networks map[string]*network
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := &ExtensionBackend{
		sm:       sm,
		extIface: extIface,
		networks: make(map[string]*network),
	}

	return be, nil
}

func (_ *ExtensionBackend) Run(ctx context.Context) {
	<-ctx.Done()
}

func (be *ExtensionBackend) RegisterNetwork(ctx context.Context, wg *sync.WaitGroup, config *subnet.Config) (backend.Network, error) {
	n := &network{
		extIface: be.extIface,
		sm:       be.sm,
	}

	// Parse out configuration
	if len(config.Backend) > 0 {
		cfg := struct {
			PreStartupCommand   string
			PostStartupCommand  string
			SubnetAddCommand    string
			SubnetRemoveCommand string
		}{}
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding backend config: %v", err)
		}
		n.preStartupCommand = cfg.PreStartupCommand
		n.postStartupCommand = cfg.PostStartupCommand
		n.subnetAddCommand = cfg.SubnetAddCommand
		n.subnetRemoveCommand = cfg.SubnetRemoveCommand
	}

	data := []byte{}
	if len(n.preStartupCommand) > 0 {
		cmd_output, err := runCmd([]string{}, "", "sh", "-c", n.preStartupCommand)
		if err != nil {
			return nil, fmt.Errorf("failed to run command: %s Err: %v Output: %s", n.preStartupCommand, err, cmd_output)
		} else {
			log.Infof("Ran command: %s\n Output: %s", n.preStartupCommand, cmd_output)
		}

		data, err = json.Marshal(cmd_output)
		if err != nil {
			return nil, err
		}
	} else {
		log.Infof("No pre startup command configured - skipping")
	}

	attrs := lease.LeaseAttrs{
		BackendType: "extension",
		BackendData: data,
	}

	if be.extIface.IfaceAddr != nil {
		attrs.PublicIP = ip.FromIP(be.extIface.IfaceAddr)
	}

	if be.extIface.IfaceV6Addr != nil {
		attrs.PublicIPv6 = ip.FromIP6(be.extIface.IfaceV6Addr)
	}

	lease, err := be.sm.AcquireLease(ctx, &attrs)
	switch err {
	case nil:
		n.lease = lease

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	if len(n.postStartupCommand) > 0 {
		cmd_output, err := runCmd([]string{
			fmt.Sprintf("NETWORK=%s", config.Network),
			fmt.Sprintf("SUBNET=%s", lease.Subnet),
			fmt.Sprintf("IPV6SUBNET=%s", lease.IPv6Subnet),
			fmt.Sprintf("PUBLIC_IP=%s", attrs.PublicIP),
			fmt.Sprintf("PUBLIC_IPV6=%s", attrs.PublicIPv6)},
			"", "sh", "-c", n.postStartupCommand)
		if err != nil {
			return nil, fmt.Errorf("failed to run command: %s Err: %v Output: %s", n.postStartupCommand, err, cmd_output)
		} else {
			log.Infof("Ran command: %s\n Output: %s", n.postStartupCommand, cmd_output)
		}
	} else {
		log.Infof("No post startup command configured - skipping")
	}

	return n, nil
}

// Run a cmd, returning a combined stdout and stderr.
func runCmd(env []string, stdin string, name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	cmd.Env = append(os.Environ(), env...)

	stdinpipe, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	_, err = io.WriteString(stdinpipe, stdin)
	if err != nil {
		return "", err
	}

	_, err = io.WriteString(stdinpipe, "\n")
	if err != nil {
		return "", err
	}
	stdinpipe.Close()

	output, err := cmd.CombinedOutput()

	return strings.TrimSpace(string(output)), err
}
