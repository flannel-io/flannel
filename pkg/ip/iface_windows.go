// Copyright 2015 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ip

import (
	"net"

	"github.com/thxcode/winnet/pkg/adapter"
)

func GetIfaceIP4Addr(iface *net.Interface) (net.IP, error) {
	// get ip address for the interface
	// prefer global unicast to link local addresses
	ifaceDetails, err := adapter.GetInterfaceByName(iface.Name)
	if err != nil {
		return nil, err
	}

	ifAddr := net.ParseIP(ifaceDetails.IpAddress)

	return ifAddr, nil
}

func GetDefaultGatewayIface() (*net.Interface, error) {
	defaultIfaceName, err := adapter.GetDefaultGatewayIfaceName()
	if err != nil {
		return nil, err
	}

	iface, err := net.InterfaceByName(defaultIfaceName)
	if err != nil {
		return nil, err
	}

	return iface, nil
}

func GetInterfaceByIP(ip net.IP) (*net.Interface, error) {
	ifaceDetails, err := adapter.GetInterfaceByIP(ip.String())
	if err != nil {
		return nil, err
	}

	iface, err := net.InterfaceByName(ifaceDetails.Name)
	if err != nil {
		return nil, err
	}

	return iface, nil
}
