// +build integration

package hcn

import (
	"testing"
)

func TestMissingNetworkByName(t *testing.T) {
	_, err := GetNetworkByName("Not found name")
	if err == nil {
		t.Fatal("Error was not thrown.")
	}
	if !IsNotFoundError(err) {
		t.Fatal("Unrelated error was thrown.")
	}
	if _, ok := err.(NetworkNotFoundError); !ok {
		t.Fatal("Wrong error type was thrown.")
	}
}

func TestMissingNetworkById(t *testing.T) {
	// Random guid
	_, err := GetNetworkByID("5f0b1190-63be-4e0c-b974-bd0f55675a42")
	if err == nil {
		t.Fatal("Unrelated error was thrown.")
	}
	if !IsNotFoundError(err) {
		t.Fatal("Unrelated error was thrown.")
	}
	if _, ok := err.(NetworkNotFoundError); !ok {
		t.Fatal("Wrong error type was thrown.")
	}
}
