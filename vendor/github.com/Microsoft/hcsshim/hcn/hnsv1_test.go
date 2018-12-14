// +build integration

package hcn

import (
	"os"
	"testing"

	"github.com/Microsoft/hcsshim"
)

const (
	NatTestNetworkName     string = "GoTestNat"
	NatTestEndpointName    string = "GoTestNatEndpoint"
	OverlayTestNetworkName string = "GoTestOverlay"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func CreateTestNetwork() (*hcsshim.HNSNetwork, error) {
	network := &hcsshim.HNSNetwork{
		Type: "NAT",
		Name: NatTestNetworkName,
		Subnets: []hcsshim.Subnet{
			{
				AddressPrefix:  "192.168.100.0/24",
				GatewayAddress: "192.168.100.1",
			},
		},
	}

	return network.Create()
}

func TestEndpoint(t *testing.T) {

	network, err := CreateTestNetwork()
	if err != nil {
		t.Fatal(err)
	}

	Endpoint := &hcsshim.HNSEndpoint{
		Name: NatTestEndpointName,
	}

	Endpoint, err = network.CreateEndpoint(Endpoint)
	if err != nil {
		t.Fatal(err)
	}

	err = Endpoint.HostAttach(1)
	if err != nil {
		t.Fatal(err)
	}

	err = Endpoint.HostDetach()
	if err != nil {
		t.Fatal(err)
	}

	_, err = Endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}

	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEndpointGetAll(t *testing.T) {
	_, err := hcsshim.HNSListEndpointRequest()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNetworkGetAll(t *testing.T) {
	_, err := hcsshim.HNSListNetworkRequest("GET", "", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestNetwork(t *testing.T) {
	network, err := CreateTestNetwork()
	if err != nil {
		t.Fatal(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}
