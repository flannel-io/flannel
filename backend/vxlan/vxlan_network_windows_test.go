// +build windows

// Copyright 2015 CNI authors
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
package vxlan

import (
	"encoding/json"
	"github.com/Microsoft/hcsshim"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	remoteEndpointJson = `
    {
        "ActivityId":  "193b6f8d-b760-4940-9abb-66a91e5af49a",
        "EncapOverhead":  50,
        "ID":  "8f109901-9970-4fda-bb2b-e9882fb3e8fb",
        "IPAddress":  "10.244.2.141",
        "IsRemoteEndpoint":  true,
        "MacAddress":  "0E-2A-0a-f4-02-8d",
        "Name":  "remote_10.244.2.141",
        "Policies":  [
                         {
                             "PA":  "10.123.74.74",
                             "Type":  "PA"
                         }
                     ],
        "PrefixLength":  16,
        "Resources":  {
                          "AllocationOrder":  1,
                          "Allocators":  [
                                             {
                                                 "AllocationOrder":  0,
                                                 "CA":  "10.244.2.141",
                                                 "ID":  "c11177da-9d65-4f81-bd76-b11d1a055b38",
                                                 "IsLocal":  false,
                                                 "IsPolicy":  true,
                                                 "PA":  "10.123.74.74",
                                                 "Tag":  "VNET Policy",
                                                 "Type":  3
                                             }
                                         ],
                          "ID":  "193b6f8d-b760-4940-9abb-66a91e5af49a",
                          "PortOperationTime":  0,
                          "State":  1,
                          "SwitchOperationTime":  0,
                          "VfpOperationTime":  0,
                          "parentId":  "e469c3a9-20ee-4de5-8fc2-d8ac99a49b18"
                      },
        "SharedContainers":  [

                             ],
        "State":  1,
        "Type":  "Overlay",
        "Version":  21474836481,
        "VirtualNetwork":  "9d59c0df-b1e2-4eee-9fde-b7ea829fc6a1",
        "VirtualNetworkName":  "vxlan0"
    }
	`
)

func TestCheckPAAddressMatch(t *testing.T) {
	endpointJson := []byte(remoteEndpointJson)
	var hnsEndpoint hcsshim.HNSEndpoint
	err := json.Unmarshal(endpointJson, &hnsEndpoint)
	assert.Nil(t, err)
	assert.True(t, checkPAAddress(&hnsEndpoint, "10.123.74.74"))
}

func TestCheckPAAddressNoMatch(t *testing.T) {
	endpointJson := []byte(remoteEndpointJson)
	var hnsEndpoint hcsshim.HNSEndpoint
	err := json.Unmarshal(endpointJson, &hnsEndpoint)
	assert.Nil(t, err)
	assert.False(t, checkPAAddress(&hnsEndpoint, "someOtherAddress"))
}

func TestCheckPAAddressNoMatchIfNil(t *testing.T) {
	var hnsEndpoint hcsshim.HNSEndpoint
	assert.False(t, checkPAAddress(&hnsEndpoint, "someOtherAddress"))
}
