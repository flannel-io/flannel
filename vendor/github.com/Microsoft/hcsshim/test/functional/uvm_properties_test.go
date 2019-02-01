// +build functional uvmproperties

package functional

import (
	"os"
	"testing"

	"github.com/Microsoft/hcsshim/internal/schema1"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/test/functional/utilities"
)

func TestPropertiesGuestConnection_LCOW(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)

	uvm := testutilities.CreateLCOWUVM(t, t.Name())
	defer uvm.Close()

	p, err := uvm.ComputeSystem().Properties(schema1.PropertyTypeGuestConnection)
	if err != nil {
		t.Fatalf("Failed to query properties: %s", err)
	}

	if p.GuestConnectionInfo.GuestDefinedCapabilities.NamespaceAddRequestSupported ||
		!p.GuestConnectionInfo.GuestDefinedCapabilities.SignalProcessSupported ||
		p.GuestConnectionInfo.ProtocolVersion < 4 {
		t.Fatalf("unexpected values: %+v", p.GuestConnectionInfo)
	}
}

func TestPropertiesGuestConnection_WCOW(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	uvm, _, uvmScratchDir := testutilities.CreateWCOWUVM(t, t.Name(), "microsoft/nanoserver")
	defer os.RemoveAll(uvmScratchDir)
	defer uvm.Close()

	p, err := uvm.ComputeSystem().Properties(schema1.PropertyTypeGuestConnection)
	if err != nil {
		t.Fatalf("Failed to query properties: %s", err)
	}

	if !p.GuestConnectionInfo.GuestDefinedCapabilities.NamespaceAddRequestSupported ||
		!p.GuestConnectionInfo.GuestDefinedCapabilities.SignalProcessSupported ||
		p.GuestConnectionInfo.ProtocolVersion < 4 {
		t.Fatalf("unexpected values: %+v", p.GuestConnectionInfo)
	}
}
