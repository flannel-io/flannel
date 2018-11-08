// +build integration

package hcn

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCreateDeleteNamespace(t *testing.T) {
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Error(err)
	}

	jsonString, err := json.Marshal(namespace)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("Namespace JSON:\n%s \n", jsonString)

	_, err = namespace.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestGetNamespaceById(t *testing.T) {
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Error(err)
	}

	foundNamespace, err := GetNamespaceByID(namespace.Id)
	if err != nil {
		t.Error(err)
	}
	if foundNamespace == nil {
		t.Errorf("No namespace found")
	}

	_, err = namespace.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestListNamespaces(t *testing.T) {
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Error(err)
	}

	foundNamespaces, err := ListNamespaces()
	if err != nil {
		t.Error(err)
	}
	if len(foundNamespaces) == 0 {
		t.Errorf("No Namespaces found")
	}

	_, err = namespace.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestGetNamespaceEndpointIds(t *testing.T) {
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
	foundEndpoints, err := GetNamespaceEndpointIds(namespace.Id)
	if err != nil {
		t.Error(err)
	}
	if len(foundEndpoints) == 0 {
		t.Errorf("No Endpoint found")
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

func TestGetNamespaceContainers(t *testing.T) {
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Error(err)
	}

	foundEndpoints, err := GetNamespaceContainerIds(namespace.Id)
	if err != nil {
		t.Error(err)
	}
	if len(foundEndpoints) != 0 {
		t.Errorf("Found containers when none should exist")
	}

	_, err = namespace.Delete()
	if err != nil {
		t.Error(err)
	}
}

func TestAddRemoveNamespaceEndpoint(t *testing.T) {
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

	err = AddNamespaceEndpoint(namespace.Id, endpoint.Id)
	if err != nil {
		t.Error(err)
	}
	foundEndpoints, err := GetNamespaceEndpointIds(namespace.Id)
	if err != nil {
		t.Error(err)
	}
	if len(foundEndpoints) == 0 {
		t.Errorf("No Endpoint found")
	}
	err = RemoveNamespaceEndpoint(namespace.Id, endpoint.Id)
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

func TestModifyNamespaceSettings(t *testing.T) {
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

	mapA := map[string]string{"EndpointId": endpoint.Id}
	settingsJson, err := json.Marshal(mapA)
	if err != nil {
		t.Error(err)
	}
	requestMessage := &ModifyNamespaceSettingRequest{
		ResourceType: NamespaceResourceTypeEndpoint,
		RequestType:  RequestTypeAdd,
		Settings:     settingsJson,
	}

	err = ModifyNamespaceSettings(namespace.Id, requestMessage)
	if err != nil {
		t.Error(err)
	}
	foundEndpoints, err := GetNamespaceEndpointIds(namespace.Id)
	if err != nil {
		t.Error(err)
	}
	if len(foundEndpoints) == 0 {
		t.Errorf("No Endpoint found")
	}
	err = RemoveNamespaceEndpoint(namespace.Id, endpoint.Id)
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
