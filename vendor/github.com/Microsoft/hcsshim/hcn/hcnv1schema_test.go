// +build integration

package hcn

import (
	"encoding/json"
	"testing"

	"github.com/Microsoft/hcsshim"
)

func TestV1Network(t *testing.T) {
	cleanup(NatTestNetworkName)

	v1network := hcsshim.HNSNetwork{
		Type: "NAT",
		Name: NatTestNetworkName,
		MacPools: []hcsshim.MacPool{
			{
				StartMacAddress: "00-15-5D-52-C0-00",
				EndMacAddress:   "00-15-5D-52-CF-FF",
			},
		},
		Subnets: []hcsshim.Subnet{
			{
				AddressPrefix:  "192.168.100.0/24",
				GatewayAddress: "192.168.100.1",
			},
		},
	}

	jsonString, err := json.Marshal(v1network)
	if err != nil {
		t.Fatal(err)
		t.Fail()
	}

	network, err := createNetwork(string(jsonString))
	if err != nil {
		t.Fatal(err)
		t.Fail()
	}

	err = network.Delete()
	if err != nil {
		t.Fatal(err)
		t.Fail()
	}
}

func TestV1Endpoint(t *testing.T) {
	cleanup(NatTestNetworkName)

	v1network := hcsshim.HNSNetwork{
		Type: "NAT",
		Name: NatTestNetworkName,
		MacPools: []hcsshim.MacPool{
			{
				StartMacAddress: "00-15-5D-52-C0-00",
				EndMacAddress:   "00-15-5D-52-CF-FF",
			},
		},
		Subnets: []hcsshim.Subnet{
			{
				AddressPrefix:  "192.168.100.0/24",
				GatewayAddress: "192.168.100.1",
			},
		},
	}

	jsonString, err := json.Marshal(v1network)
	if err != nil {
		t.Fatal(err)
		t.Fail()
	}

	network, err := createNetwork(string(jsonString))
	if err != nil {
		t.Fatal(err)
		t.Fail()
	}

	v1endpoint := hcsshim.HNSEndpoint{
		Name:           NatTestEndpointName,
		VirtualNetwork: network.Id,
	}

	jsonString, err = json.Marshal(v1endpoint)
	if err != nil {
		t.Fatal(err)
		t.Fail()
	}

	endpoint, err := createEndpoint(network.Id, string(jsonString))
	if err != nil {
		t.Fatal(err)
		t.Fail()
	}

	err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
		t.Fail()
	}

	err = network.Delete()
	if err != nil {
		t.Fatal(err)
		t.Fail()
	}
}
