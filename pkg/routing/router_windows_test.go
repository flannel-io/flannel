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

package routing

import (
	"net"
	"testing"

	"github.com/flannel-io/flannel/pkg/ip"
)

func TestGetAllRoutes(t *testing.T) {
	router := RouterWindows{}
	routes, err := router.GetAllRoutes()
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) == 0 {
		t.Fatal("len(routes) == 0")
	}
}

func TestCreateAndRemoveRoute(t *testing.T) {
	defaultInterface, err := ip.GetDefaultGatewayInterface()
	if err != nil {
		t.Fatal(err)
	}

	router := RouterWindows{}
	allRoutes, err := router.GetAllRoutes()
	if err != nil {
		t.Fatal(err)
	}
	if len(allRoutes) == 0 {
		t.Fatal("len(routes) == 0")
	}

	destinationSubnet := allRoutes[0].DestinationSubnet
	gatewayAddress := net.ParseIP("192.168.199.123")

	routes, err := router.GetRoutesFromInterfaceToSubnet(defaultInterface.Index, destinationSubnet)
	if err != nil {
		t.Fatal(err)
	}

	firstRouteLen := len(routes)
	if len(routes) == 0 {
		t.Fatal("len(routes) == 0")
	}

	err = router.CreateRoute(defaultInterface.Index, destinationSubnet, gatewayAddress)
	if err != nil {
		t.Fatal(err)
	}

	routes, err = router.GetRoutesFromInterfaceToSubnet(defaultInterface.Index, destinationSubnet)
	if err != nil {
		t.Fatal(err)
	}

	expectedLen := firstRouteLen + 1
	givenLen := len(routes)
	if givenLen != expectedLen {
		t.Fatalf("givenLen:%d != expectedLen:%d", givenLen, expectedLen)
	}

	err = router.DeleteRoute(defaultInterface.Index, destinationSubnet, gatewayAddress)
	if err != nil {
		t.Fatal(err)
	}

	routes, err = router.GetRoutesFromInterfaceToSubnet(defaultInterface.Index, destinationSubnet)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != firstRouteLen {
		t.Fatal("len(routes) != startRouteLen")
	}
}
