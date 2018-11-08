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
		t.Error(err)
	}
	fmt.Printf("Supported Features:\n%s \n", jsonString)
}

func TestV2ApiSupport(t *testing.T) {
	err := V2ApiSupported()
	if err != nil {
		t.Error(err)
	}
}
