// +build integration

package hcn

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCreateDeleteNetwork(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	jsonString, err := json.Marshal(network)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Network JSON:\n%s \n", jsonString)
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetNetworkByName(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	network, err = GetNetworkByName(network.Name)
	if err != nil {
		t.Fatal(err)
	}
	if network == nil {
		t.Fatal("No Network found")
	}
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetNetworkById(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	network, err = GetNetworkByID(network.Id)
	if err != nil {
		t.Fatal(err)
	}
	if network == nil {
		t.Fatal("No Network found")
	}
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestListNetwork(t *testing.T) {
	_, err := ListNetworks()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddRemoveRemoteSubnetRoutePolicy(t *testing.T) {

	network, err := CreateTestOverlayNetwork()
	if err != nil {
		t.Fatal(err)
	}

	remoteSubnetRoutePolicy, err := HcnCreateTestRemoteSubnetRoute()
	if err != nil {
		t.Fatal(err)
	}

	//Add Policy
	network.AddPolicy(*remoteSubnetRoutePolicy)

	//Reload the network object from HNS.
	network, err = GetNetworkByID(network.Id)
	if err != nil {
		t.Fatal(err)
	}

	foundPolicy := false
	for _, policy := range network.Policies {
		if policy.Type == RemoteSubnetRoute {
			foundPolicy = true
			break
		}
	}
	if !foundPolicy {
		t.Fatalf("Could not find remote subnet route policy on network.")
	}

	//Remove Policy.
	network.RemovePolicy(*remoteSubnetRoutePolicy)

	//Reload the network object from HNS.
	network, err = GetNetworkByID(network.Id)
	if err != nil {
		t.Fatal(err)
	}

	foundPolicy = false
	for _, policy := range network.Policies {
		if policy.Type == RemoteSubnetRoute {
			foundPolicy = true
			break
		}
	}
	if foundPolicy {
		t.Fatalf("Found remote subnet route policy on network when it should have been deleted.")
	}

	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}
