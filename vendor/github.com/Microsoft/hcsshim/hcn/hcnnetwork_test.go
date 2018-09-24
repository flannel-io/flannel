// +build integration

package hcn

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCreateDeleteNetwork(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	jsonString, err := json.Marshal(network)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("Network JSON:\n%s \n", jsonString)
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestGetNetworkByName(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	network, err = GetNetworkByName(network.Name)
	if err != nil {
		t.Error(err)
	}
	if network == nil {
		t.Errorf("No Network found")
	}
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestGetNetworkById(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	network, err = GetNetworkByID(network.Id)
	if err != nil {
		t.Error(err)
	}
	if network == nil {
		t.Errorf("No Network found")
	}
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestListNetwork(t *testing.T) {
	_, err := ListNetworks()
	if err != nil {
		t.Error(err)
	}
}
