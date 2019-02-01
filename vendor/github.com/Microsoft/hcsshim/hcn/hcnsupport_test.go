// +build integration

package hcn

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestSupportedFeatures(t *testing.T) {
	supportedFeatures := GetSupportedFeatures()
	jsonString, err := json.Marshal(supportedFeatures)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Supported Features:\n%s \n", jsonString)
}

func TestV2ApiSupport(t *testing.T) {
	supportedFeatures := GetSupportedFeatures()
	err := V2ApiSupported()
	if supportedFeatures.Api.V2 && err != nil {
		t.Fatal(err)
	}
	if !supportedFeatures.Api.V2 && err == nil {
		t.Fatal(err)
	}
}

func TestRemoteSubnetSupport(t *testing.T) {
	supportedFeatures := GetSupportedFeatures()
	err := RemoteSubnetSupported()
	if supportedFeatures.RemoteSubnet && err != nil {
		t.Fatal(err)
	}
	if !supportedFeatures.RemoteSubnet && err == nil {
		t.Fatal(err)
	}
}

func TestHostRouteSupport(t *testing.T) {
	supportedFeatures := GetSupportedFeatures()
	err := HostRouteSupported()
	if supportedFeatures.HostRoute && err != nil {
		t.Fatal(err)
	}
	if !supportedFeatures.HostRoute && err == nil {
		t.Fatal(err)
	}
}

func TestDSRSupport(t *testing.T) {
	supportedFeatures := GetSupportedFeatures()
	err := DSRSupported()
	if supportedFeatures.DSR && err != nil {
		t.Fatal(err)
	}
	if !supportedFeatures.DSR && err == nil {
		t.Fatal(err)
	}
}
