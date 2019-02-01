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

	err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
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
	err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
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

	err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = secondEndpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
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

	loadBalancer, err := AddLoadBalancer([]HostComputeEndpoint{*endpoint}, LoadBalancerFlagsNone, LoadBalancerPortMappingFlagsNone, "10.0.0.1", []string{"1.1.1.2", "1.1.1.3"}, 6, 8080, 80)
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

	err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
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

	portMappings := LoadBalancerPortMappingFlagsPreserveDIP | LoadBalancerPortMappingFlagsUseMux
	loadBalancer, err := AddLoadBalancer([]HostComputeEndpoint{*endpoint}, LoadBalancerFlagsDSR, portMappings, "10.0.0.1", []string{"1.1.1.2", "1.1.1.3"}, 6, 8080, 80)
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
	if foundLB.Flags != 1 {
		t.Fatal(fmt.Errorf("IsDSR is not set"))
	}

	foundFlags := foundLB.PortMappings[0].Flags
	if foundFlags&LoadBalancerPortMappingFlagsUseMux == 0 {
		t.Fatal(fmt.Errorf("UseMux is not set"))
	}
	if foundFlags&LoadBalancerPortMappingFlagsPreserveDIP == 0 {
		t.Fatal(fmt.Errorf("PreserveDIP is not set"))
	}

	err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
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

	loadBalancer, err := AddLoadBalancer([]HostComputeEndpoint{*endpoint}, LoadBalancerFlagsNone, LoadBalancerPortMappingFlagsILB, "10.0.0.1", []string{"1.1.1.2", "1.1.1.3"}, 6, 8080, 80)
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

	foundFlags := foundLB.PortMappings[0].Flags
	if foundFlags&LoadBalancerPortMappingFlagsILB == 0 {
		t.Fatal(fmt.Errorf("Loadbalancer is not ILB"))
	}

	err = loadBalancer.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}
