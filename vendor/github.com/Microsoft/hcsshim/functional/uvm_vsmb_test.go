// +build functional uvmvsmb

package functional

import (
	"os"
	"testing"

	"github.com/Microsoft/hcsshim/functional/utilities"
	"github.com/Microsoft/hcsshim/internal/osversion"
	"github.com/Microsoft/hcsshim/internal/schema2"
)

// TestVSMB tests adding/removing VSMB layers from a v2 Windows utility VM
func TestVSMB(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	nanoLayers := testutilities.LayerFolders(t, "microsoft/nanoserver")

	uvm, uvmScratchDir := testutilities.CreateWCOWUVM(t, nanoLayers, "", nil)
	defer os.RemoveAll(uvmScratchDir)
	defer uvm.Terminate()

	dir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(dir)
	var iterations uint32 = 64
	options := &hcsschema.VirtualSmbShareOptions{
		ReadOnly:            true,
		PseudoOplocks:       true,
		TakeBackupPrivilege: true,
		CacheIo:             true,
		ShareRead:           true,
	}
	for i := 0; i < int(iterations); i++ {
		if err := uvm.AddVSMB(dir, "", options); err != nil {
			t.Fatalf("AddVSMB failed: %s", err)
		}
	}

	// Remove them all
	for i := 0; i < int(iterations); i++ {
		if err := uvm.RemoveVSMB(dir); err != nil {
			t.Fatalf("RemoveVSMB failed: %s", err)
		}
	}
}

// TODO: VSMB for mapped directories
