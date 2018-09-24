// +build integration

package hcn

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCreateDeleteEndpoint(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}
	jsonString, err := json.Marshal(Endpoint)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("Endpoint JSON:\n%s \n", jsonString)

	_, err = Endpoint.Delete()
	if err != nil {
		t.Error(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestGetEndpointById(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}

	foundEndpoint, err := GetEndpointByID(Endpoint.Id)
	if err != nil {
		t.Error(err)
	}
	if foundEndpoint == nil {
		t.Errorf("No Endpoint found")
	}

	_, err = foundEndpoint.Delete()
	if err != nil {
		t.Error(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestGetEndpointByName(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}

	foundEndpoint, err := GetEndpointByName(Endpoint.Name)
	if err != nil {
		t.Error(err)
	}
	if foundEndpoint == nil {
		t.Errorf("No Endpoint found")
	}

	_, err = foundEndpoint.Delete()
	if err != nil {
		t.Error(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestListEndpoints(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}

	foundEndpoints, err := ListEndpoints()
	if err != nil {
		t.Error(err)
	}
	if len(foundEndpoints) == 0 {
		t.Errorf("No Endpoint found")
	}

	_, err = Endpoint.Delete()
	if err != nil {
		t.Error(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestListEndpointsOfNetwork(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}

	foundEndpoints, err := ListEndpointsOfNetwork(network.Id)
	if err != nil {
		t.Error(err)
	}
	if len(foundEndpoints) == 0 {
		t.Errorf("No Endpoint found")
	}

	_, err = Endpoint.Delete()
	if err != nil {
		t.Error(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestEndpointNamespaceAttachDetach(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Error(err)
	}

	err = endpoint.NamespaceAttach(namespace.Id)
	if err != nil {
		t.Error(err)
	}
	err = endpoint.NamespaceDetach(namespace.Id)
	if err != nil {
		t.Error(err)
	}

	_, err = namespace.Delete()
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

func TestCreateEndpointWithNamespace(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Error(err)
	}
	Endpoint, err := HcnCreateTestEndpointWithNamespace(network, namespace)
	if err != nil {
		t.Error(err)
	}
	if Endpoint.HostComputeNamespace == "" {
		t.Errorf("No Namespace detected.")
	}

	_, err = Endpoint.Delete()
	if err != nil {
		t.Error(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestApplyPolicyOnEndpoint(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	Endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}
	endpointPolicyList, err := HcnCreateAcls()
	if err != nil {
		t.Error(err)
	}
	jsonString, err := json.Marshal(*endpointPolicyList)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("ACLS JSON:\n%s \n", jsonString)
	err = Endpoint.ApplyPolicy(*endpointPolicyList)
	if err != nil {
		t.Error(err)
	}

	foundEndpoint, err := GetEndpointByName(Endpoint.Name)
	if err != nil {
		t.Error(err)
	}
	if len(foundEndpoint.Policies) == 0 {
		t.Errorf("No Endpoint Policies found")
	}

	_, err = Endpoint.Delete()
	if err != nil {
		t.Error(err)
	}
	_, err = network.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestModifyEndpointSettings(t *testing.T) {
	network, err := HcnCreateTestNetwork()
	if err != nil {
		t.Error(err)
	}
	endpoint, err := HcnCreateTestEndpoint(network)
	if err != nil {
		t.Error(err)
	}
	endpointPolicy, err := HcnCreateAcls()
	if err != nil {
		t.Error(err)
	}
	settingsJson, err := json.Marshal(endpointPolicy)
	if err != nil {
		t.Error(err)
	}

	requestMessage := &ModifyEndpointSettingRequest{
		ResourceType: EndpointResourceTypePolicy,
		RequestType:  RequestTypeUpdate,
		Settings:     settingsJson,
	}

	err = ModifyEndpointSettings(endpoint.Id, requestMessage)
	if err != nil {
		t.Error(err)
	}

	foundEndpoint, err := GetEndpointByName(endpoint.Name)
	if err != nil {
		t.Error(err)
	}
	if len(foundEndpoint.Policies) == 0 {
		t.Errorf("No Endpoint Policies found")
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
