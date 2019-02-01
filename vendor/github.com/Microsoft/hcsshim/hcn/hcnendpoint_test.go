// +build integration

package hcn

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCreateDeleteEndpoint(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}
	jsonString, err := json.Marshal(Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Endpoint JSON:\n%s \n", jsonString)

	err = Endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetEndpointById(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}

	foundEndpoint, err := GetEndpointByID(Endpoint.Id)
	if err != nil {
		t.Fatal(err)
	}
	if foundEndpoint == nil {
		t.Fatal("No Endpoint found")
	}

	err = foundEndpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetEndpointByName(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}

	foundEndpoint, err := GetEndpointByName(Endpoint.Name)
	if err != nil {
		t.Fatal(err)
	}
	if foundEndpoint == nil {
		t.Fatal("No Endpoint found")
	}

	err = foundEndpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestListEndpoints(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}

	foundEndpoints, err := ListEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if len(foundEndpoints) == 0 {
		t.Fatal("No Endpoint found")
	}

	err = Endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestListEndpointsOfNetwork(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}

	foundEndpoints, err := ListEndpointsOfNetwork(network.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(foundEndpoints) == 0 {
		t.Fatal("No Endpoint found")
	}

	err = Endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEndpointNamespaceAttachDetach(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Fatal(err)
	}

	err = endpoint.NamespaceAttach(namespace.Id)
	if err != nil {
		t.Fatal(err)
	}
	err = endpoint.NamespaceDetach(namespace.Id)
	if err != nil {
		t.Fatal(err)
	}

	err = namespace.Delete()
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

func TestCreateEndpointWithNamespace(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Fatal(err)
	}
	Endpoint, err := HcnCreateTestEndpointWithNamespace(network, namespace)
	if err != nil {
		t.Fatal(err)
	}
	if Endpoint.HostComputeNamespace == "" {
		t.Fatal("No Namespace detected.")
	}

	err = Endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyPolicyOnEndpoint(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}
	endpointPolicyList, err := HcnCreateAcls()
	if err != nil {
		t.Fatal(err)
	}
	jsonString, err := json.Marshal(*endpointPolicyList)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("ACLS JSON:\n%s \n", jsonString)
	err = Endpoint.ApplyPolicy(*endpointPolicyList)
	if err != nil {
		t.Fatal(err)
	}

	foundEndpoint, err := GetEndpointByName(Endpoint.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(foundEndpoint.Policies) == 0 {
		t.Fatal("No Endpoint Policies found")
	}

	err = Endpoint.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestModifyEndpointSettings(t *testing.T) {
	network, err := HcnCreateTestNATNetwork()
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Fatal(err)
	}
	endpointPolicy, err := HcnCreateAcls()
	if err != nil {
		t.Fatal(err)
	}
	settingsJson, err := json.Marshal(endpointPolicy)
	if err != nil {
		t.Fatal(err)
	}

	requestMessage := &ModifyEndpointSettingRequest{
		ResourceType: EndpointResourceTypePolicy,
		RequestType:  RequestTypeUpdate,
		Settings:     settingsJson,
	}

	err = ModifyEndpointSettings(endpoint.Id, requestMessage)
	if err != nil {
		t.Fatal(err)
	}

	foundEndpoint, err := GetEndpointByName(endpoint.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(foundEndpoint.Policies) == 0 {
		t.Fatal("No Endpoint Policies found")
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
