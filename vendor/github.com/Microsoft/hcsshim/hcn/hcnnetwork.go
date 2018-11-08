package hcn

import (
	"encoding/json"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/interop"
	"github.com/sirupsen/logrus"
)

// Route is assoicated with a subnet.
type Route struct {
	NextHop           string `json:",omitempty"`
	DestinationPrefix string `json:",omitempty"`
	Metric            uint16 `json:",omitempty"`
}

// Subnet is assoicated with a Ipam.
type Subnet struct {
	IpAddressPrefix string            `json:",omitempty"`
	Policies        []json.RawMessage `json:",omitempty"`
	Routes          []Route           `json:",omitempty"`
}

// Ipam (Internet Protocol Addres Management) is assoicated with a network
// and represents the address space(s) of a network.
type Ipam struct {
	Type    string   `json:",omitempty"` // Ex: Static, DHCP
	Subnets []Subnet `json:",omitempty"`
}

// MacRange is associated with MacPool and respresents the start and end addresses.
type MacRange struct {
	StartMacAddress string `json:",omitempty"`
	EndMacAddress   string `json:",omitempty"`
}

// MacPool is assoicated with a network and represents pool of MacRanges.
type MacPool struct {
	Ranges []MacRange `json:",omitempty"`
}

// Dns (Domain Name System is associated with a network.
type Dns struct {
	Suffix     string   `json:",omitempty"`
	Search     []string `json:",omitempty"`
	ServerList []string `json:",omitempty"`
	Options    []string `json:",omitempty"`
}

// HostComputeNetwork represents a network
type HostComputeNetwork struct {
	Id            string          `json:"ID,omitempty"`
	Name          string          `json:",omitempty"`
	Type          string          `json:",omitempty"` // EX: NAT, Overlay
	Policies      []NetworkPolicy `json:",omitempty"`
	MacPool       MacPool         `json:",omitempty"`
	Dns           Dns             `json:",omitempty"`
	Ipams         []Ipam          `json:",omitempty"`
	Flags         uint32          `json:",omitempty"` // 0: None
	SchemaVersion SchemaVersion   `json:",omitempty"`
}

func getNetwork(networkGuid guid.GUID, query string) (*HostComputeNetwork, error) {
	// Open network.
	var (
		networkHandle    hcnNetwork
		resultBuffer     *uint16
		propertiesBuffer *uint16
	)
	hr := hcnOpenNetwork(&networkGuid, &networkHandle, &resultBuffer)
	if err := checkForErrors("hcnOpenNetwork", hr, resultBuffer); err != nil {
		return nil, err
	}
	// Query network.
	hr = hcnQueryNetworkProperties(networkHandle, query, &propertiesBuffer, &resultBuffer)
	if err := checkForErrors("hcnQueryNetworkProperties", hr, resultBuffer); err != nil {
		return nil, err
	}
	properties := interop.ConvertAndFreeCoTaskMemString(propertiesBuffer)
	// Close network.
	hr = hcnCloseNetwork(networkHandle)
	if err := checkForErrors("hcnCloseNetwork", hr, nil); err != nil {
		return nil, err
	}
	// Convert output to HostComputeNetwork
	var outputNetwork HostComputeNetwork
	if err := json.Unmarshal([]byte(properties), &outputNetwork); err != nil {
		return nil, err
	}
	return &outputNetwork, nil
}

func enumerateNetworks(query string) ([]HostComputeNetwork, error) {
	// Enumerate all Network Guids
	var (
		resultBuffer  *uint16
		networkBuffer *uint16
	)
	hr := hcnEnumerateNetworks(query, &networkBuffer, &resultBuffer)
	if err := checkForErrors("hcnEnumerateNetworks", hr, resultBuffer); err != nil {
		return nil, err
	}

	networks := interop.ConvertAndFreeCoTaskMemString(networkBuffer)
	var networkIds []guid.GUID
	if err := json.Unmarshal([]byte(networks), &networkIds); err != nil {
		return nil, err
	}

	var outputNetworks []HostComputeNetwork
	for _, networkGuid := range networkIds {
		network, err := getNetwork(networkGuid, query)
		if err != nil {
			return nil, err
		}
		outputNetworks = append(outputNetworks, *network)
	}
	return outputNetworks, nil
}

func createNetwork(settings string) (*HostComputeNetwork, error) {
	// Create new network.
	var (
		networkHandle    hcnNetwork
		resultBuffer     *uint16
		propertiesBuffer *uint16
	)
	networkGuid := guid.GUID{}
	hr := hcnCreateNetwork(&networkGuid, settings, &networkHandle, &resultBuffer)
	if err := checkForErrors("hcnCreateNetwork", hr, resultBuffer); err != nil {
		return nil, err
	}
	// Query network.
	hcnQuery := defaultQuery()
	query, err := json.Marshal(hcnQuery)
	if err != nil {
		return nil, err
	}
	hr = hcnQueryNetworkProperties(networkHandle, string(query), &propertiesBuffer, &resultBuffer)
	if err := checkForErrors("hcnQueryNetworkProperties", hr, resultBuffer); err != nil {
		return nil, err
	}
	properties := interop.ConvertAndFreeCoTaskMemString(propertiesBuffer)
	// Close network.
	hr = hcnCloseNetwork(networkHandle)
	if err := checkForErrors("hcnCloseNetwork", hr, nil); err != nil {
		return nil, err
	}
	// Convert output to HostComputeNetwork
	var outputNetwork HostComputeNetwork
	if err := json.Unmarshal([]byte(properties), &outputNetwork); err != nil {
		return nil, err
	}
	return &outputNetwork, nil
}

func modifyNetwork(networkId string, settings string) (*HostComputeNetwork, error) {
	networkGuid := guid.FromString(networkId)
	// Open Network
	var (
		networkHandle    hcnNetwork
		resultBuffer     *uint16
		propertiesBuffer *uint16
	)
	hr := hcnOpenNetwork(&networkGuid, &networkHandle, &resultBuffer)
	if err := checkForErrors("hcnOpenNetwork", hr, resultBuffer); err != nil {
		return nil, err
	}
	// Modify Network
	hr = hcnModifyNetwork(networkHandle, settings, &resultBuffer)
	if err := checkForErrors("hcnModifyNetwork", hr, resultBuffer); err != nil {
		return nil, err
	}
	// Query network.
	hcnQuery := defaultQuery()
	query, err := json.Marshal(hcnQuery)
	if err != nil {
		return nil, err
	}
	hr = hcnQueryNetworkProperties(networkHandle, string(query), &propertiesBuffer, &resultBuffer)
	if err := checkForErrors("hcnQueryNetworkProperties", hr, resultBuffer); err != nil {
		return nil, err
	}
	properties := interop.ConvertAndFreeCoTaskMemString(propertiesBuffer)
	// Close network.
	hr = hcnCloseNetwork(networkHandle)
	if err := checkForErrors("hcnCloseNetwork", hr, nil); err != nil {
		return nil, err
	}
	// Convert output to HostComputeNetwork
	var outputNetwork HostComputeNetwork
	if err := json.Unmarshal([]byte(properties), &outputNetwork); err != nil {
		return nil, err
	}
	return &outputNetwork, nil
}

func deleteNetwork(networkId string) error {
	networkGuid := guid.FromString(networkId)
	var resultBuffer *uint16
	hr := hcnDeleteNetwork(&networkGuid, &resultBuffer)
	if err := checkForErrors("hcnDeleteNetwork", hr, resultBuffer); err != nil {
		return err
	}
	return nil
}

// ListNetworks makes a call to list all available networks.
func ListNetworks() ([]HostComputeNetwork, error) {
	hcnQuery := defaultQuery()
	networks, err := ListNetworksQuery(hcnQuery)
	if err != nil {
		return nil, err
	}
	return networks, nil
}

// ListNetworksQuery makes a call to query the list of available networks.
func ListNetworksQuery(query HostComputeQuery) ([]HostComputeNetwork, error) {
	queryJson, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	networks, err := enumerateNetworks(string(queryJson))
	if err != nil {
		return nil, err
	}
	return networks, nil
}

// GetNetworkByID returns the network specified by Id.
func GetNetworkByID(networkID string) (*HostComputeNetwork, error) {
	hcnQuery := defaultQuery()
	mapA := map[string]string{"ID": networkID}
	filter, err := json.Marshal(mapA)
	if err != nil {
		return nil, err
	}
	hcnQuery.Filter = string(filter)

	networks, err := ListNetworksQuery(hcnQuery)
	if err != nil {
		return nil, err
	}
	if len(networks) == 0 {
		return nil, fmt.Errorf("Network %s not found", networkID)
	}
	return &networks[0], err
}

// GetNetworkByName returns the network specified by Name.
func GetNetworkByName(networkName string) (*HostComputeNetwork, error) {
	hcnQuery := defaultQuery()
	mapA := map[string]string{"Name": networkName}
	filter, err := json.Marshal(mapA)
	if err != nil {
		return nil, err
	}
	hcnQuery.Filter = string(filter)

	networks, err := ListNetworksQuery(hcnQuery)
	if err != nil {
		return nil, err
	}
	if len(networks) == 0 {
		return nil, fmt.Errorf("Network %s not found", networkName)
	}
	return &networks[0], err
}

// Create Network.
func (network *HostComputeNetwork) Create() (*HostComputeNetwork, error) {
	logrus.Debugf("hcn::HostComputeNetwork::Create id=%s", network.Id)

	jsonString, err := json.Marshal(network)
	if err != nil {
		return nil, err
	}

	network, hcnErr := createNetwork(string(jsonString))
	if hcnErr != nil {
		return nil, hcnErr
	}
	return network, nil
}

// Delete Network.
func (network *HostComputeNetwork) Delete() (*HostComputeNetwork, error) {
	logrus.Debugf("hcn::HostComputeNetwork::Delete id=%s", network.Id)

	if err := deleteNetwork(network.Id); err != nil {
		return nil, err
	}
	return nil, nil
}

// CreateEndpoint creates an endpoint on the Network.
func (network *HostComputeNetwork) CreateEndpoint(endpoint *HostComputeEndpoint) (*HostComputeEndpoint, error) {
	isRemote := endpoint.Flags&EndpointFlagsRemoteEndpoint != 0
	logrus.Debugf("hcn::HostComputeNetwork::CreatEndpoint, networkId=%s remote=%t", network.Id, isRemote)

	endpoint.HostComputeNetwork = network.Id
	endpointSettings, err := json.Marshal(endpoint)
	if err != nil {
		return nil, err
	}
	newEndpoint, err := createEndpoint(network.Id, string(endpointSettings))
	if err != nil {
		return nil, err
	}
	return newEndpoint, nil
}

// CreateRemoteEndpoint creates a remote endpoint on the Network.
func (network *HostComputeNetwork) CreateRemoteEndpoint(endpoint *HostComputeEndpoint) (*HostComputeEndpoint, error) {
	endpoint.Flags = EndpointFlagsRemoteEndpoint | endpoint.Flags
	return network.CreateEndpoint(endpoint)
}
