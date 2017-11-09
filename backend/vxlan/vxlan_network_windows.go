// +build windows

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

package vxlan

import (
	"sync"

	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/subnet"

	"encoding/json"
	"fmt"
	"github.com/Microsoft/hcsshim"
	"github.com/coreos/flannel/pkg/ip"
	"net"
	"time"
)

type network struct {
	name      string
	networkId string
	macPrefix string
	extIface  *backend.ExternalInterface
	lease     *subnet.Lease
	sm        subnet.Manager
}

func (n *network) Lease() *subnet.Lease {
	return n.lease
}

func (n *network) MTU() int {
	return n.extIface.Iface.MTU
}

func (n *network) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.Info("Watching for new subnet leases")
	evts := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, n.sm, n.lease, evts)
		wg.Done()
	}()

	defer wg.Wait()

	for {
		select {
		case evtBatch := <-evts:
			n.handleSubnetEvents(evtBatch)

		case <-ctx.Done():
			return
		}
	}
}

func conjureMac(macPrefix string, ip ip.IP4) string {
	a, b, c, d := ip.Octets()
	return fmt.Sprintf("%v-%02x-%02x-%02x-%02x", macPrefix, a, b, c, d)
}

func (n *network) handleSubnetEvents(batch []subnet.Event) {
	for _, evt := range batch {
		if evt.Lease.Attrs.BackendType != "vxlan" {
			log.Warningf("Ignoring non-vxlan subnet: type=%v", evt.Lease.Attrs.BackendType)
			continue
		}

		if evt.Type != subnet.EventAdded && evt.Type != subnet.EventRemoved {
			log.Error("Internal error: unknown event type: ", int(evt.Type))
			continue
		}

		// add or delete all possible remote IPs (excluding gateway & bcast) as remote endpoints
		managementIp := evt.Lease.Attrs.PublicIP.String()
		lastIP := evt.Lease.Subnet.Next().IP - 1

		start := time.Now()
		for remoteIp := evt.Lease.Subnet.IP + 2; remoteIp < lastIP; remoteIp++ {
			remoteMac := conjureMac(n.macPrefix, remoteIp)
			remoteEndpointName := fmt.Sprintf("remote_%v", remoteIp.String())

			if evt.Type == subnet.EventAdded {
				if err := createRemoteEndpoint(remoteEndpointName, remoteIp, remoteMac, managementIp, n.networkId); err != nil {
					log.Errorf("failed to create remote endpoint [%v], error: %v", remoteEndpointName, err)
				}
			} else {
				if hnsEndpoint, err := hcsshim.GetHNSEndpointByName(remoteEndpointName); err != nil {
					if _, err := hnsEndpoint.Delete(); err != nil {
						log.Errorf("unable to delete existing remote endpoint [%v], error: %v", remoteEndpointName, err)
					}
				}
			}
		}

		t := time.Now()
		elapsed := t.Sub(start)

		message := "Subnet removed"
		if evt.Type == subnet.EventAdded {
			message = "Subnet added"
		}
		log.Infof("%v: %v [%v ns]", message, evt.Lease.Subnet, elapsed.Nanoseconds())
	}
}

func checkPAAddress(hnsEndpoint *hcsshim.HNSEndpoint, managementAddress string) bool {
	if hnsEndpoint.Policies == nil {
		return false
	}

	for _, policyJson := range hnsEndpoint.Policies {
		var policy map[string]interface{}
		if json.Unmarshal(policyJson, &policy) != nil {
			return false
		}

		if valType, ok := policy["Type"]; ok && valType.(string) == "PA" {
			if val, ok := policy["PA"]; ok {
				if val.(string) == managementAddress {
					return true
				}
			}
		}
	}

	return false
}

func createRemoteEndpoint(remoteEndpointName string, remoteIp ip.IP4, remoteMac string, managementAddress string, networkId string) error {

	// find existing
	hnsEndpoint, err := hcsshim.GetHNSEndpointByName(remoteEndpointName)
	if err == nil && hnsEndpoint.VirtualNetwork == networkId && checkPAAddress(hnsEndpoint, managementAddress) {
		return nil
	}

	// create or replace endpoint
	if hnsEndpoint != nil {
		if _, err = hnsEndpoint.Delete(); err != nil {
			log.Errorf("unable to delete existing remote endpoint [%v], error: %v", remoteEndpointName, err)
			return err
		}
	}

	paPolicy := struct {
		Type string
		PA   string
	}{
		Type: "PA",
		PA:   managementAddress,
	}

	policyBytes, _ := json.Marshal(&paPolicy)

	hnsEndpoint = &hcsshim.HNSEndpoint{
		Id:               "",
		Name:             remoteEndpointName,
		IPAddress:        net.IPv4(remoteIp.Octets()),
		MacAddress:       remoteMac,
		VirtualNetwork:   networkId,
		IsRemoteEndpoint: true,
		Policies: []json.RawMessage{
			policyBytes,
		},
	}

	hnsEndpoint, err = hnsEndpoint.Create()
	if err != nil {
		log.Errorf("unable to create remote endpoint [%v], error: %v", remoteEndpointName, err)
		return err
	}

	return nil
}
