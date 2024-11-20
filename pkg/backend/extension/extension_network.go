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
	"sync"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/flannel-io/flannel/pkg/subnet"
	log "k8s.io/klog/v2"
)

type network struct {
	extIface            *backend.ExternalInterface
	lease               *lease.Lease
	sm                  subnet.Manager
	preStartupCommand   string
	postStartupCommand  string
	subnetAddCommand    string
	subnetRemoveCommand string
}

func (n *network) Lease() *lease.Lease {
	return n.lease
}

func (n *network) MTU() int {
	return n.extIface.Iface.MTU
}

func (n *network) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.Info("Watching for new subnet leases")
	evts := make(chan []lease.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, n.sm, n.lease, evts)
		wg.Done()
	}()

	defer wg.Wait()

	for {
		evtBatch, ok := <-evts
		if !ok {
			log.Infof("evts chan closed")
			return
		}
		n.handleSubnetEvents(evtBatch)
	}
}

func (n *network) handleSubnetEvents(batch []lease.Event) {
	for _, evt := range batch {
		switch evt.Type {
		case lease.EventAdded:
			log.Infof("Subnet added: %v via %v", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP)

			if evt.Lease.Attrs.BackendType != "extension" {
				log.Warningf("Ignoring non-extension subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			if len(n.subnetAddCommand) > 0 {
				backendData := ""

				if len(evt.Lease.Attrs.BackendData) > 0 {
					if err := json.Unmarshal(evt.Lease.Attrs.BackendData, &backendData); err != nil {
						log.Errorf("failed to unmarshal BackendData: %v", err)
						continue
					}
				}

				cmd_output, err := runCmd([]string{
					fmt.Sprintf("SUBNET=%s", evt.Lease.Subnet),
					fmt.Sprintf("PUBLIC_IP=%s", evt.Lease.Attrs.PublicIP)},
					backendData,
					"sh", "-c", n.subnetAddCommand)

				if err != nil {
					log.Errorf("failed to run command: %s Err: %v Output: %s", n.subnetAddCommand, err, cmd_output)
				} else {
					log.Infof("Ran command: %s\n Output: %s", n.subnetAddCommand, cmd_output)
				}
			}

		case lease.EventRemoved:
			log.Info("Subnet removed: ", evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "extension" {
				log.Warningf("Ignoring non-extension subnet: type=%v", evt.Lease.Attrs.BackendType)
				continue
			}

			if len(n.subnetRemoveCommand) > 0 {
				backendData := ""

				if len(evt.Lease.Attrs.BackendData) > 0 {
					if err := json.Unmarshal(evt.Lease.Attrs.BackendData, &backendData); err != nil {
						log.Errorf("failed to unmarshal BackendData: %v", err)
						continue
					}
				}
				cmd_output, err := runCmd([]string{
					fmt.Sprintf("SUBNET=%s", evt.Lease.Subnet),
					fmt.Sprintf("PUBLIC_IP=%s", evt.Lease.Attrs.PublicIP)},
					backendData,
					"sh", "-c", n.subnetRemoveCommand)

				if err != nil {
					log.Errorf("failed to run command: %s Err: %v Output: %s", n.subnetRemoveCommand, err, cmd_output)
				} else {
					log.Infof("Ran command: %s\n Output: %s", n.subnetRemoveCommand, cmd_output)
				}
			}

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}
