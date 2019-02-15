package uvm

import (
	"fmt"
	"path"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/Microsoft/hcsshim/internal/guestrequest"
	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/hns"
	"github.com/Microsoft/hcsshim/internal/requesttype"
	"github.com/Microsoft/hcsshim/internal/schema1"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/sirupsen/logrus"
)

// AddNetNS adds network namespace inside the guest & adds endpoints to the guest on that namepace
func (uvm *UtilityVM) AddNetNS(id string, endpoints []*hns.HNSEndpoint) (err error) {
	uvm.m.Lock()
	defer uvm.m.Unlock()
	ns := uvm.namespaces[id]
	if ns == nil {
		ns = &namespaceInfo{}

		if uvm.isNetworkNamespaceSupported() {
			// Add a Guest Network namespace. On LCOW we add the adapters
			// dynamically.
			if uvm.operatingSystem == "windows" {
				hcnNamespace, err := hcn.GetNamespaceByID(id)
				if err != nil {
					return err
				}
				guestNamespace := hcsschema.ModifySettingRequest{
					GuestRequest: guestrequest.GuestRequest{
						ResourceType: guestrequest.ResourceTypeNetworkNamespace,
						RequestType:  requesttype.Add,
						Settings:     hcnNamespace,
					},
				}
				if err := uvm.Modify(&guestNamespace); err != nil {
					return err
				}
			}
		}

		defer func() {
			if err != nil {
				if e := uvm.removeNamespaceNICs(ns); e != nil {
					logrus.Warnf("failed to undo NIC add: %v", e)
				}
			}
		}()
		for _, endpoint := range endpoints {
			nicID := guid.New()
			err = uvm.addNIC(nicID, endpoint)
			if err != nil {
				return err
			}
			ns.nics = append(ns.nics, nicInfo{nicID, endpoint})
		}
		if uvm.namespaces == nil {
			uvm.namespaces = make(map[string]*namespaceInfo)
		}
		uvm.namespaces[id] = ns
	}
	ns.refCount++
	return nil
}

//RemoveNetNS removes the namespace information
func (uvm *UtilityVM) RemoveNetNS(id string) error {
	uvm.m.Lock()
	defer uvm.m.Unlock()
	ns := uvm.namespaces[id]
	if ns == nil || ns.refCount <= 0 {
		panic(fmt.Errorf("removed a namespace that was not added: %s", id))
	}

	ns.refCount--

	// Remove the Guest Network namespace
	if uvm.isNetworkNamespaceSupported() {
		if uvm.operatingSystem == "windows" {
			hcnNamespace, err := hcn.GetNamespaceByID(id)
			if err != nil {
				return err
			}
			guestNamespace := hcsschema.ModifySettingRequest{
				GuestRequest: guestrequest.GuestRequest{
					ResourceType: guestrequest.ResourceTypeNetworkNamespace,
					RequestType:  requesttype.Remove,
					Settings:     hcnNamespace,
				},
			}
			if err := uvm.Modify(&guestNamespace); err != nil {
				return err
			}
		}
	}

	var err error
	if ns.refCount == 0 {
		err = uvm.removeNamespaceNICs(ns)
		delete(uvm.namespaces, id)
	}

	return err
}

// IsNetworkNamespaceSupported returns bool value specifying if network namespace is supported inside the guest
func (uvm *UtilityVM) isNetworkNamespaceSupported() bool {
	p, err := uvm.ComputeSystem().Properties(schema1.PropertyTypeGuestConnection)
	if err == nil {
		return p.GuestConnectionInfo.GuestDefinedCapabilities.NamespaceAddRequestSupported
	}

	return false
}

func (uvm *UtilityVM) removeNamespaceNICs(ns *namespaceInfo) error {
	for len(ns.nics) != 0 {
		nic := ns.nics[len(ns.nics)-1]
		err := uvm.removeNIC(nic.ID, nic.Endpoint)
		if err != nil {
			return err
		}
		ns.nics = ns.nics[:len(ns.nics)-1]
	}
	return nil
}

func getNetworkModifyRequest(adapterID string, requestType string, settings interface{}) interface{} {
	if osversion.Get().Build >= osversion.RS5 {
		return guestrequest.NetworkModifyRequest{
			AdapterId:   adapterID,
			RequestType: requestType,
			Settings:    settings,
		}
	}
	return guestrequest.RS4NetworkModifyRequest{
		AdapterInstanceId: adapterID,
		RequestType:       requestType,
		Settings:          settings,
	}
}

func (uvm *UtilityVM) addNIC(id guid.GUID, endpoint *hns.HNSEndpoint) error {

	// First a pre-add. This is a guest-only request and is only done on Windows.
	if uvm.operatingSystem == "windows" {
		preAddRequest := hcsschema.ModifySettingRequest{
			GuestRequest: guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeNetwork,
				RequestType:  requesttype.Add,
				Settings: getNetworkModifyRequest(
					id.String(),
					requesttype.PreAdd,
					endpoint),
			},
		}
		if err := uvm.Modify(&preAddRequest); err != nil {
			return err
		}
	}

	// Then the Add itself
	request := hcsschema.ModifySettingRequest{
		RequestType:  requesttype.Add,
		ResourcePath: path.Join("VirtualMachine/Devices/NetworkAdapters", id.String()),
		Settings: hcsschema.NetworkAdapter{
			EndpointId: endpoint.Id,
			MacAddress: endpoint.MacAddress,
		},
	}

	if uvm.operatingSystem == "windows" {
		request.GuestRequest = guestrequest.GuestRequest{
			ResourceType: guestrequest.ResourceTypeNetwork,
			RequestType:  requesttype.Add,
			Settings: getNetworkModifyRequest(
				id.String(),
				requesttype.Add,
				nil),
		}
	} else {
		// Verify this version of LCOW supports Network HotAdd
		if uvm.isNetworkNamespaceSupported() {
			request.GuestRequest = guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeNetwork,
				RequestType:  requesttype.Add,
				Settings: &guestrequest.LCOWNetworkAdapter{
					NamespaceID:     endpoint.Namespace.ID,
					ID:              id.String(),
					MacAddress:      endpoint.MacAddress,
					IPAddress:       endpoint.IPAddress.String(),
					PrefixLength:    endpoint.PrefixLength,
					GatewayAddress:  endpoint.GatewayAddress,
					DNSSuffix:       endpoint.DNSSuffix,
					DNSServerList:   endpoint.DNSServerList,
					EnableLowMetric: endpoint.EnableLowMetric,
					EncapOverhead:   endpoint.EncapOverhead,
				},
			}
		}
	}

	if err := uvm.Modify(&request); err != nil {
		return err
	}

	return nil
}

func (uvm *UtilityVM) removeNIC(id guid.GUID, endpoint *hns.HNSEndpoint) error {
	request := hcsschema.ModifySettingRequest{
		RequestType:  requesttype.Remove,
		ResourcePath: path.Join("VirtualMachine/Devices/NetworkAdapters", id.String()),
		Settings: hcsschema.NetworkAdapter{
			EndpointId: endpoint.Id,
			MacAddress: endpoint.MacAddress,
		},
	}

	if uvm.operatingSystem == "windows" {
		request.GuestRequest = hcsschema.ModifySettingRequest{
			RequestType: requesttype.Remove,
			Settings: getNetworkModifyRequest(
				id.String(),
				requesttype.Remove,
				nil),
		}
	} else {
		// Verify this version of LCOW supports Network HotRemove
		if uvm.isNetworkNamespaceSupported() {
			request.GuestRequest = guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeNetwork,
				RequestType:  requesttype.Remove,
				Settings: &guestrequest.LCOWNetworkAdapter{
					NamespaceID: endpoint.Namespace.ID,
					ID:          endpoint.Id,
				},
			}
		}
	}

	if err := uvm.Modify(&request); err != nil {
		return err
	}
	return nil
}
