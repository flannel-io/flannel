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

package hostgw

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/Microsoft/hcsshim"
	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
	"github.com/golang/glog"
	netsh "github.com/rakelkar/gonetsh/netsh"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/util/json"
)

func init() {
	backend.Register("host-gw", New)
}

const (
	routeCheckRetries = 10
)

type HostgwBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
	networks map[string]*network
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	if !extIface.ExtAddr.Equal(extIface.IfaceAddr) {
		return nil, fmt.Errorf("your PublicIP differs from interface IP, meaning that probably you're on a NAT, which is not supported by host-gw backend")
	}

	be := &HostgwBackend{
		sm:       sm,
		extIface: extIface,
		networks: make(map[string]*network),
	}

	return be, nil
}

func (be *HostgwBackend) RegisterNetwork(ctx context.Context, wg sync.WaitGroup, config *subnet.Config) (backend.Network, error) {
	n := &network{
		extIface:  be.extIface,
		sm:        be.sm,
		name:      be.extIface.Iface.Name,
		linkIndex: be.extIface.Iface.Index,
	}

	attrs := subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(be.extIface.ExtAddr),
		BackendType: "host-gw",
	}

	l, err := be.sm.AcquireLease(ctx, &attrs)
	switch err {
	case nil:
		n.lease = l

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	backendConfig := struct {
		networkName   string
		dnsServerList string
	}{
		networkName: "cbr0",
	}

	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &backendConfig); err != nil {
			return nil, fmt.Errorf("error decoding windows HOST-GW backend config: %v", err)
		}
	}

	if backendConfig.networkName == "" {
		return nil, fmt.Errorf("Backend.networkName is required e.g. cbr0")
	}

	// check if the network exists and has the expected settings?
	var networkId string
	createNetwork := true
	addressPrefix := n.lease.Subnet.String()
	networkGatewayAddress := n.lease.Subnet.IP + 1
	podGatewayAddress := n.lease.Subnet.IP + 2
	hnsNetwork, err := hcsshim.GetHNSNetworkByName(backendConfig.networkName)
	if err == nil && hnsNetwork.DNSServerList == backendConfig.dnsServerList {
		for _, subnet := range hnsNetwork.Subnets {
			if subnet.AddressPrefix == addressPrefix && subnet.GatewayAddress == networkGatewayAddress.String() {
				networkId = hnsNetwork.Id
				createNetwork = false
				glog.Infof("Found existing HNS network [%+v]", hnsNetwork)
				break
			}
		}
	}

	if createNetwork {
		// create, but a network with the same name exists?
		if hnsNetwork != nil {
			if _, err := hnsNetwork.Delete(); err != nil {
				return nil, fmt.Errorf("unable to delete existing network [%v], error: %v", hnsNetwork.Name, err)
			}
			glog.Infof("Deleted stale HNS network [%v]")
		}

		// create the underlying windows HNS network
		request := map[string]interface{}{
			"Name": backendConfig.networkName,
			"Type": "l2bridge",
			"Subnets": []interface{}{
				map[string]interface{}{
					"AddressPrefix":  addressPrefix,
					"GatewayAddress": networkGatewayAddress,
				},
			},
			"DNSServerList": backendConfig.dnsServerList,
		}

		jsonRequest, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}

		glog.Infof("Attempting to create HNS network, request: %v", string(jsonRequest))
		newHnsNetwork, err := hcsshim.HNSNetworkRequest("POST", "", string(jsonRequest))
		if err != nil {
			return nil, fmt.Errorf("unable to create network [%v], error: %v", backendConfig.networkName, err)
		}

		hnsNetwork = newHnsNetwork
		networkId = hnsNetwork.Id
		glog.Infof("Created HNS network [%v] as %+v", backendConfig.networkName, hnsNetwork)
	}

	// now ensure there is a 1.2 endpoint on this network in the host compartment
	var endpointToAttach *hcsshim.HNSEndpoint
	bridgeEndpointName := backendConfig.networkName + "_ep"
	createEndpoint := true
	hnsEndpoint, err := hcsshim.GetHNSEndpointByName(bridgeEndpointName)
	if err == nil && hnsEndpoint.IPAddress.String() == podGatewayAddress.String() {
		glog.Infof("Found existing HNS bridge endpoint [%+v]", hnsEndpoint)
		endpointToAttach = hnsEndpoint
		createEndpoint = false
	}

	if createEndpoint {
		if hnsEndpoint != nil {
			if _, err = hnsEndpoint.Delete(); err != nil {
				return nil, fmt.Errorf("unable to delete existing bridge endpoint [%v], error: %v", bridgeEndpointName, err)
			}
			glog.Infof("Deleted stale HNS endpoint [%v]")
		}

		hnsEndpoint = &hcsshim.HNSEndpoint{
			Id:             "",
			Name:           bridgeEndpointName,
			IPAddress:      podGatewayAddress.ToIP(),
			VirtualNetwork: networkId,
		}

		glog.Infof("Attempting to create HNS endpoint [%+v]", hnsEndpoint)
		hnsEndpoint, err = hnsEndpoint.Create()
		if err != nil {
			return nil, fmt.Errorf("unable to create bridge endpoint [%v], error: %v", bridgeEndpointName, err)
		}
		endpointToAttach = hnsEndpoint
		glog.Infof("Created bridge endpoint [%v] as %+v", bridgeEndpointName, hnsEndpoint)
	}
	if err = endpointToAttach.HostAttach(1); err != nil {
		return nil, fmt.Errorf("unable to hot attach bridge endpoint [%v] to host compartment, error: %v", bridgeEndpointName, err)
	}
	glog.Infof("Attached bridge endpoint [%v] to host", bridgeEndpointName)

	// enable forwarding on the host interface and endpoint
	netHelper := netsh.New(nil)
	for _, interfaceIpAddress := range []string{hnsNetwork.ManagementIP, endpointToAttach.IPAddress.String()} {
		netInterface, err := netHelper.GetInterfaceByIP(interfaceIpAddress)
		if err != nil {
			return nil, fmt.Errorf("unable to find interface for IP Addess [%v], error: %v", interfaceIpAddress, err)
		}

		interfaceIdx := strconv.Itoa(netInterface.Idx)
		if err := netHelper.EnableForwarding(interfaceIdx); err != nil {
			return nil, fmt.Errorf("unable to enable forwarding on [%v] index [%v], error: %v", netInterface.Name, interfaceIdx, err)
		}
		glog.Infof("Enabled forwarding on [%v] index [%v]", netInterface.Name, interfaceIdx)
	}

	return n, nil
}
