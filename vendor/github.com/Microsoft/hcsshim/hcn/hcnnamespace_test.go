// +build integration

package hcn

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Microsoft/hcsshim/internal/cni"
	"github.com/Microsoft/hcsshim/internal/guid"
)

func TestNewNamespace(t *testing.T) {
	_ = NewNamespace(NamespaceTypeHost)
	_ = NewNamespace(NamespaceTypeHostDefault)
	_ = NewNamespace(NamespaceTypeGuest)
	_ = NewNamespace(NamespaceTypeGuestDefault)
}

func TestCreateDeleteNamespace(t *testing.T) {
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Fatal(err)
	}

	jsonString, err := json.Marshal(namespace)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Namespace JSON:\n%s \n", jsonString)

	err = namespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateDeleteNamespaceGuest(t *testing.T) {
	namespace := &HostComputeNamespace{
		Type: NamespaceTypeGuestDefault,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	hnsNamespace, err := namespace.Create()
	if err != nil {
		t.Fatal(err)
	}

	err = hnsNamespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetNamespaceById(t *testing.T) {
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Fatal(err)
	}

	foundNamespace, err := GetNamespaceByID(namespace.Id)
	if err != nil {
		t.Fatal(err)
	}
	if foundNamespace == nil {
		t.Fatal("No namespace found")
	}

	err = namespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestListNamespaces(t *testing.T) {
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Fatal(err)
	}

	foundNamespaces, err := ListNamespaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(foundNamespaces) == 0 {
		t.Fatal("No Namespaces found")
	}

	err = namespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetNamespaceEndpointIds(t *testing.T) {
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
	foundEndpoints, err := GetNamespaceEndpointIds(namespace.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(foundEndpoints) == 0 {
		t.Fatal("No Endpoint found")
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

func TestGetNamespaceContainers(t *testing.T) {
	namespace, err := HcnCreateTestNamespace()
	if err != nil {
		t.Fatal(err)
	}

	foundEndpoints, err := GetNamespaceContainerIds(namespace.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(foundEndpoints) != 0 {
		t.Fatal("Found containers when none should exist")
	}

	err = namespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddRemoveNamespaceEndpoint(t *testing.T) {
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

	err = AddNamespaceEndpoint(namespace.Id, endpoint.Id)
	if err != nil {
		t.Fatal(err)
	}
	foundEndpoints, err := GetNamespaceEndpointIds(namespace.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(foundEndpoints) == 0 {
		t.Fatal("No Endpoint found")
	}
	err = RemoveNamespaceEndpoint(namespace.Id, endpoint.Id)
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

func TestModifyNamespaceSettings(t *testing.T) {
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

	mapA := map[string]string{"EndpointId": endpoint.Id}
	settingsJson, err := json.Marshal(mapA)
	if err != nil {
		t.Fatal(err)
	}
	requestMessage := &ModifyNamespaceSettingRequest{
		ResourceType: NamespaceResourceTypeEndpoint,
		RequestType:  RequestTypeAdd,
		Settings:     settingsJson,
	}

	err = ModifyNamespaceSettings(namespace.Id, requestMessage)
	if err != nil {
		t.Fatal(err)
	}
	foundEndpoints, err := GetNamespaceEndpointIds(namespace.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(foundEndpoints) == 0 {
		t.Fatal("No Endpoint found")
	}
	err = RemoveNamespaceEndpoint(namespace.Id, endpoint.Id)
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

// Sync Tests

func TestSyncNamespaceHostDefault(t *testing.T) {
	namespace := &HostComputeNamespace{
		Type:        NamespaceTypeHostDefault,
		NamespaceId: 5,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	hnsNamespace, err := namespace.Create()
	if err != nil {
		t.Fatal(err)
	}

	// Host namespace types should be no-op success
	err = hnsNamespace.Sync()
	if err != nil {
		t.Fatal(err)
	}

	err = hnsNamespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyncNamespaceHost(t *testing.T) {
	namespace := &HostComputeNamespace{
		Type:        NamespaceTypeHost,
		NamespaceId: 5,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	hnsNamespace, err := namespace.Create()
	if err != nil {
		t.Fatal(err)
	}

	// Host namespace types should be no-op success
	err = hnsNamespace.Sync()
	if err != nil {
		t.Fatal(err)
	}

	err = hnsNamespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyncNamespaceGuestNoReg(t *testing.T) {
	namespace := &HostComputeNamespace{
		Type:        NamespaceTypeGuest,
		NamespaceId: 5,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	hnsNamespace, err := namespace.Create()
	if err != nil {
		t.Fatal(err)
	}

	// Guest namespace type with out reg state should be no-op success
	err = hnsNamespace.Sync()
	if err != nil {
		t.Fatal(err)
	}

	err = hnsNamespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyncNamespaceGuestDefaultNoReg(t *testing.T) {
	namespace := &HostComputeNamespace{
		Type:        NamespaceTypeGuestDefault,
		NamespaceId: 5,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	hnsNamespace, err := namespace.Create()
	if err != nil {
		t.Fatal(err)
	}

	// Guest namespace type with out reg state should be no-op success
	err = hnsNamespace.Sync()
	if err != nil {
		t.Fatal(err)
	}

	err = hnsNamespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyncNamespaceGuest(t *testing.T) {
	namespace := &HostComputeNamespace{
		Type:        NamespaceTypeGuest,
		NamespaceId: 5,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	hnsNamespace, err := namespace.Create()

	if err != nil {
		t.Fatal(err)
	}

	// Create registry state
	pnc := cni.NewPersistedNamespaceConfig(t.Name(), "test-container", guid.New())
	err = pnc.Store()
	if err != nil {
		pnc.Remove()
		t.Fatal(err)
	}

	// Guest namespace type with reg state but not Vm shim should pass...
	// after trying to connect to VM shim that it doesn't find and remove the Key so it doesn't look again.
	err = hnsNamespace.Sync()
	if err != nil {
		t.Fatal(err)
	}

	err = pnc.Remove()
	if err != nil {
		t.Fatal(err)
	}
	err = hnsNamespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyncNamespaceGuestDefault(t *testing.T) {
	namespace := &HostComputeNamespace{
		Type:        NamespaceTypeGuestDefault,
		NamespaceId: 5,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	hnsNamespace, err := namespace.Create()
	if err != nil {
		t.Fatal(err)
	}

	// Create registry state
	pnc := cni.NewPersistedNamespaceConfig(t.Name(), "test-container", guid.New())
	err = pnc.Store()
	if err != nil {
		pnc.Remove()
		t.Fatal(err)
	}

	// Guest namespace type with reg state but not Vm shim should pass...
	// after trying to connect to VM shim that it doesn't find and remove the Key so it doesn't look again.
	err = hnsNamespace.Sync()
	if err != nil {
		t.Fatal(err)
	}

	err = pnc.Remove()
	if err != nil {
		t.Fatal(err)
	}
	err = hnsNamespace.Delete()
	if err != nil {
		t.Fatal(err)
	}
}
