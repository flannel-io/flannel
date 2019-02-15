// +build integration

package hcn

import (
	"encoding/json"
)

func cleanup(networkName string) {
	// Delete test network (if exists)
	testNetwork, err := GetNetworkByName(networkName)
	if err != nil {
		return
	}
	if testNetwork != nil {
		err := testNetwork.Delete()
		if err != nil {
			return
		}
	}
}

func HcnCreateTestNATNetwork() (*HostComputeNetwork, error) {
	cleanup(NatTestNetworkName)
	network := &HostComputeNetwork{
		Type: "NAT",
		Name: NatTestNetworkName,
		MacPool: MacPool{
			Ranges: []MacRange{
				{
					StartMacAddress: "00-15-5D-52-C0-00",
					EndMacAddress:   "00-15-5D-52-CF-FF",
				},
			},
		},
		Ipams: []Ipam{
			{
				Type: "Static",
				Subnets: []Subnet{
					{
						IpAddressPrefix: "192.168.100.0/24",
						Routes: []Route{
							{
								NextHop:           "192.168.100.1",
								DestinationPrefix: "0.0.0.0",
							},
						},
					},
				},
			},
		},
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	return network.Create()
}

func CreateTestOverlayNetwork() (*HostComputeNetwork, error) {
	cleanup(OverlayTestNetworkName)
	network := &HostComputeNetwork{
		Type: "Overlay",
		Name: OverlayTestNetworkName,
		MacPool: MacPool{
			Ranges: []MacRange{
				{
					StartMacAddress: "00-15-5D-52-C0-00",
					EndMacAddress:   "00-15-5D-52-CF-FF",
				},
			},
		},
		Ipams: []Ipam{
			{
				Type: "Static",
				Subnets: []Subnet{
					{
						IpAddressPrefix: "192.168.100.0/24",
						Routes: []Route{
							{
								NextHop:           "192.168.100.1",
								DestinationPrefix: "0.0.0.0/0",
							},
						},
					},
				},
			},
		},
		Flags: EnableNonPersistent,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	vsid := &VsidPolicySetting{
		IsolationId: 5000,
	}
	vsidJson, err := json.Marshal(vsid)
	if err != nil {
		return nil, err
	}

	sp := &SubnetPolicy{
		Type: VSID,
	}
	sp.Settings = vsidJson

	spJson, err := json.Marshal(sp)
	if err != nil {
		return nil, err
	}

	network.Ipams[0].Subnets[0].Policies = append(network.Ipams[0].Subnets[0].Policies, spJson)

	return network.Create()
}

func HcnCreateTestEndpoint(network *HostComputeNetwork) (*HostComputeEndpoint, error) {
	if network == nil {

	}
	Endpoint := &HostComputeEndpoint{
		Name: NatTestEndpointName,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	return network.CreateEndpoint(Endpoint)
}

func HcnCreateTestEndpointWithNamespace(network *HostComputeNetwork, namespace *HostComputeNamespace) (*HostComputeEndpoint, error) {
	Endpoint := &HostComputeEndpoint{
		Name:                 NatTestEndpointName,
		HostComputeNamespace: namespace.Id,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	return network.CreateEndpoint(Endpoint)
}

func HcnCreateTestNamespace() (*HostComputeNamespace, error) {
	namespace := &HostComputeNamespace{
		Type:        NamespaceTypeHostDefault,
		NamespaceId: 5,
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	return namespace.Create()
}

func HcnCreateAcls() (*PolicyEndpointRequest, error) {
	in := AclPolicySetting{
		Protocols:       "6",
		Action:          ActionTypeAllow,
		Direction:       DirectionTypeIn,
		LocalAddresses:  "192.168.100.0/24,10.0.0.21",
		RemoteAddresses: "192.168.100.0/24,10.0.0.21",
		LocalPorts:      "80,8080",
		RemotePorts:     "80,8080",
		RuleType:        RuleTypeSwitch,
		Priority:        200,
	}

	rawJSON, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	inPolicy := EndpointPolicy{
		Type:     ACL,
		Settings: rawJSON,
	}

	out := AclPolicySetting{
		Protocols:       "6",
		Action:          ActionTypeAllow,
		Direction:       DirectionTypeOut,
		LocalAddresses:  "192.168.100.0/24,10.0.0.21",
		RemoteAddresses: "192.168.100.0/24,10.0.0.21",
		LocalPorts:      "80,8080",
		RemotePorts:     "80,8080",
		RuleType:        RuleTypeSwitch,
		Priority:        200,
	}

	rawJSON, err = json.Marshal(out)
	if err != nil {
		return nil, err
	}
	outPolicy := EndpointPolicy{
		Type:     ACL,
		Settings: rawJSON,
	}

	endpointRequest := PolicyEndpointRequest{
		Policies: []EndpointPolicy{inPolicy, outPolicy},
	}

	return &endpointRequest, nil
}

func HcnCreateTestLoadBalancer(endpoint *HostComputeEndpoint) (*HostComputeLoadBalancer, error) {
	loadBalancer := &HostComputeLoadBalancer{
		HostComputeEndpoints: []string{endpoint.Id},
		SourceVIP:            "10.0.0.1",
		PortMappings: []LoadBalancerPortMapping{
			{
				Protocol:     6, // TCP
				InternalPort: 8080,
				ExternalPort: 8090,
			},
		},
		FrontendVIPs: []string{"1.1.1.2", "1.1.1.3"},
		SchemaVersion: SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	return loadBalancer.Create()
}

func HcnCreateTestRemoteSubnetRoute() (*PolicyNetworkRequest, error) {
	rsr := RemoteSubnetRoutePolicySetting{
		DestinationPrefix:           "192.168.2.0/24",
		IsolationId:                 5000,
		ProviderAddress:             "1.1.1.1",
		DistributedRouterMacAddress: "00-12-34-56-78-9a",
	}

	rawJSON, err := json.Marshal(rsr)
	if err != nil {
		return nil, err
	}
	rsrPolicy := NetworkPolicy{
		Type:     RemoteSubnetRoute,
		Settings: rawJSON,
	}

	networkRequest := PolicyNetworkRequest{
		Policies: []NetworkPolicy{rsrPolicy},
	}

	return &networkRequest, nil
}

func HcnCreateTestHostRoute() (*PolicyNetworkRequest, error) {
	hostRoutePolicy := NetworkPolicy{
		Type:     HostRoute,
		Settings: []byte("{}"),
	}

	networkRequest := PolicyNetworkRequest{
		Policies: []NetworkPolicy{hostRoutePolicy},
	}

	return &networkRequest, nil
}
