// +build integration

package hcn

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCreateDeleteLoadBalancer(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}
	loadBalancer, err := HcnCreateTestLoadBalancer(endpoint)
	if err != nil {
		t.Error(err)
	}
	jsonString, err := json.Marshal(loadBalancer)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("LoadBalancer JSON:\n%s \n", jsonString)

	_, err = loadBalancer.Delete()
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

func TestGetLoadBalancerById(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}
	loadBalancer, err := HcnCreateTestLoadBalancer(endpoint)
	if err != nil {
		t.Error(err)
	}
	foundLB, err := GetLoadBalancerByID(loadBalancer.Id)
	if err != nil {
		t.Error(err)
	}
	if foundLB == nil {
		t.Errorf("No loadBalancer found")
	}
	_, err = loadBalancer.Delete()
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

func TestListLoadBalancer(t *testing.T) {
	_, err := ListLoadBalancers()
	if err != nil {
		t.Error(err)
	}
}

func TestLoadBalancerAddRemoveEndpoint(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}
	loadBalancer, err := HcnCreateTestLoadBalancer(endpoint)
	if err != nil {
		t.Error(err)
	}

	secondEndpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}
	updatedLB, err := loadBalancer.AddEndpoint(secondEndpoint)
	if err != nil {
		t.Error(err)
	}

	if len(updatedLB.HostComputeEndpoints) != 2 {
		t.Errorf("Endpoint not added to loadBalancer")
	}
	updatedLB, err = loadBalancer.RemoveEndpoint(secondEndpoint)
	if err != nil {
		t.Error(err)
	}
	if len(updatedLB.HostComputeEndpoints) != 1 {
		t.Errorf("Endpoint not removed from loadBalancer")
	}

	_, err = loadBalancer.Delete()
	if err != nil {
		t.Error(err)
	}
	_, err = secondEndpoint.Delete()
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

func TestAddLoadBalancer(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}

	loadBalancer, err := AddLoadBalancer([]HostComputeEndpoint{*endpoint}, false, "10.0.0.1", []string{"1.1.1.2", "1.1.1.3"}, 6, 8080, 80)
	if err != nil {
		t.Error(err)
	}
	foundLB, err := GetLoadBalancerByID(loadBalancer.Id)
	if err != nil {
		t.Error(err)
	}
	if foundLB == nil {
		t.Error(fmt.Errorf("No loadBalancer found"))
	}

	_, err = loadBalancer.Delete()
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
