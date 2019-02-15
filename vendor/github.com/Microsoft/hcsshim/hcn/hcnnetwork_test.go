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
	err = network.Delete()
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
	err = network.Delete()
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
	err = network.Delete()
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

func testNetworkPolicy(t *testing.T, policiesToTest *PolicyNetworkRequest) {
	network, err := CreateTestOverlayNetwork()
	if err != nil {
		t.Fatal(err)
	}

	network.AddPolicy(*policiesToTest)

	//Reload the network object from HNS.
	network, err = GetNetworkByID(network.Id)
	if err != nil {
		t.Fatal(err)
	}

	for _, policyToTest := range policiesToTest.Policies {
		foundPolicy := false
		for _, policy := range network.Policies {
			if policy.Type == policyToTest.Type {
				foundPolicy = true
				break
			}
		}
		if !foundPolicy {
			t.Fatalf("Could not find %s policy on network.", policyToTest.Type)
		}
	}

	network.RemovePolicy(*policiesToTest)

	//Reload the network object from HNS.
	network, err = GetNetworkByID(network.Id)
	if err != nil {
		t.Fatal(err)
	}

	for _, policyToTest := range policiesToTest.Policies {
		foundPolicy := false
		for _, policy := range network.Policies {
			if policy.Type == policyToTest.Type {
				foundPolicy = true
				break
			}
		}
		if foundPolicy {
			t.Fatalf("Found %s policy on network when it should have been deleted.", policyToTest.Type)
		}
	}

	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddRemoveRemoteSubnetRoutePolicy(t *testing.T) {

	remoteSubnetRoutePolicy, err := HcnCreateTestRemoteSubnetRoute()
	if err != nil {
		t.Fatal(err)
	}

	testNetworkPolicy(t, remoteSubnetRoutePolicy)
}

func TestAddRemoveHostRoutePolicy(t *testing.T) {

	hostRoutePolicy, err := HcnCreateTestHostRoute()
	if err != nil {
		t.Fatal(err)
	}

	testNetworkPolicy(t, hostRoutePolicy)
}

func TestNetworkFlags(t *testing.T) {

	network, err := CreateTestOverlayNetwork()
	if err != nil {
		t.Fatal(err)
	}

	//Reload the network object from HNS.
	network, err = GetNetworkByID(network.Id)
	if err != nil {
		t.Fatal(err)
	}

	if network.Flags != EnableNonPersistent {
		t.Errorf("EnableNonPersistent flag (%d) is not set on network", EnableNonPersistent)
	}

	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}
