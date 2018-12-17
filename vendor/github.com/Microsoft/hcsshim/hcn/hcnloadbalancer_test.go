// +build integration

package hcn

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCreateDeleteLoadBalancer(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}
	loadBalancer, err := HcnCreateTestLoadBalancer(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	jsonString, err := json.Marshal(loadBalancer)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("LoadBalancer JSON:\n%s \n", jsonString)

	_, err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetLoadBalancerById(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}
	loadBalancer, err := HcnCreateTestLoadBalancer(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	foundLB, err := GetLoadBalancerByID(loadBalancer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if foundLB == nil {
		t.Fatalf("No loadBalancer found")
	}
	_, err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestListLoadBalancer(t *testing.T) {
	_, err := ListLoadBalancers()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadBalancerAddRemoveEndpoint(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}
	loadBalancer, err := HcnCreateTestLoadBalancer(endpoint)
	if err != nil {
		t.Fatal(err)
	}

	secondEndpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}
	updatedLB, err := loadBalancer.AddEndpoint(secondEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	if len(updatedLB.HostComputeEndpoints) != 2 {
		t.Fatalf("Endpoint not added to loadBalancer")
	}
	updatedLB, err = loadBalancer.RemoveEndpoint(secondEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	if len(updatedLB.HostComputeEndpoints) != 1 {
		t.Fatalf("Endpoint not removed from loadBalancer")
	}

	_, err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = secondEndpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddLoadBalancer(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}

	loadBalancer, err := AddLoadBalancer([]HostComputeEndpoint{*endpoint}, false, false, "10.0.0.1", []string{"1.1.1.2", "1.1.1.3"}, 6, 8080, 80)
	if err != nil {
		t.Fatal(err)
	}
	foundLB, err := GetLoadBalancerByID(loadBalancer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if foundLB == nil {
		t.Fatal(fmt.Errorf("No loadBalancer found"))
	}

	_, err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddDSRLoadBalancer(t *testing.T) {
	network, err := CreateTestOverlayNetwork()
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}

	loadBalancer, err := AddLoadBalancer([]HostComputeEndpoint{*endpoint}, false, true, "10.0.0.1", []string{"1.1.1.2", "1.1.1.3"}, 6, 8080, 80)
	if err != nil {
		t.Fatal(err)
	}
	foundLB, err := GetLoadBalancerByID(loadBalancer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if foundLB == nil {
		t.Fatal(fmt.Errorf("No loadBalancer found"))
	}

	_, err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddILBLoadBalancer(t *testing.T) {
	network, err := CreateTestOverlayNetwork()
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}

	loadBalancer, err := AddLoadBalancer([]HostComputeEndpoint{*endpoint}, true, false, "10.0.0.1", []string{"1.1.1.2", "1.1.1.3"}, 6, 8080, 80)
	if err != nil {
		t.Fatal(err)
	}
	foundLB, err := GetLoadBalancerByID(loadBalancer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if foundLB == nil {
		t.Fatal(fmt.Errorf("No loadBalancer found"))
	}

	_, err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}
