// +build integration

package hcn

import (
	"encoding/json"
	"testing"

	"github.com/Microsoft/hcsshim"
)

func TestV1Network(t *testing.T) {
	cleanup()

	v1network := hcsshim.HNSNetwork{
		Type: "NAT",
		Name: NatTestNetworkName,
		MacPools: []hcsshim.MacPool{
			hcsshim.MacPool{
				StartMacAddress: "00-15-5D-52-C0-00",
				EndMacAddress:   "00-15-5D-52-CF-FF",
			},
		},
		Subnets: []hcsshim.Subnet{
			hcsshim.Subnet{
				AddressPrefix:  "192.168.100.0/24",
				GatewayAddress: "192.168.100.1",
			},
		},
	}

	jsonString, err := json.Marshal(v1network)
	if err != nil {
		t.Error(err)
	}

	network, err := createNetwork(string(jsonString))
	if err != nil {
		t.Error(err)
	}

	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestV1Endpoint(t *testing.T) {
	cleanup()

	v1network := hcsshim.HNSNetwork{
		Type: "NAT",
		Name: NatTestNetworkName,
		MacPools: []hcsshim.MacPool{
			hcsshim.MacPool{
				StartMacAddress: "00-15-5D-52-C0-00",
				EndMacAddress:   "00-15-5D-52-CF-FF",
			},
		},
		Subnets: []hcsshim.Subnet{
			hcsshim.Subnet{
				AddressPrefix:  "192.168.100.0/24",
				GatewayAddress: "192.168.100.1",
			},
		},
	}

	jsonString, err := json.Marshal(v1network)
	if err != nil {
		t.Error(err)
	}

	network, err := createNetwork(string(jsonString))
	if err != nil {
		t.Error(err)
	}

	v1endpoint := hcsshim.HNSEndpoint{
		Name:           NatTestEndpointName,
		VirtualNetwork: network.Id,
	}

	jsonString, err = json.Marshal(v1endpoint)
	if err != nil {
		t.Error(err)
	}

	endpoint, err := createEndpoint(network.Id, string(jsonString))
	if err != nil {
		t.Error(err)
	}

	_, err = endpoint.Delete()
	if err != nil {
		t.Error(err)
	}

	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}
