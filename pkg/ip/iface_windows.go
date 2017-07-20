// +build windows

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
	"errors"
	"net"
)

func GetIfaceIP4Addr(iface *net.Interface) (net.IP, error) {
	return nil, errors.New("GetIfaceIP4Addr not implemented for this platform")
}

func GetIfaceIP4AddrMatch(iface *net.Interface, matchAddr net.IP) error {
	return errors.New("GetIfaceIP4AddrMatch not implemented for this platform")
}

func GetDefaultGatewayIface() (*net.Interface, error) {
	return nil, errors.New("GetDefaultGatewayIface not implemented for this platform")
}

func GetInterfaceByIP(ip net.IP) (*net.Interface, error) {
	return nil, errors.New("GetInterfaceByIP not implemented for this platform")
}
