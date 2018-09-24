package testutilities

import (
	"path/filepath"
	"testing"

	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/lcow"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/internal/wclayer"
)

const lcowGlobalSVMID = "test.lcowglobalsvm"

var (
	lcowGlobalSVM        *uvm.UtilityVM
	lcowCacheScratchFile string
)

func init() {
	if hcsSystem, err := hcs.OpenComputeSystem(lcowGlobalSVMID); err == nil {
		hcsSystem.Terminate()
	}
}

// CreateWCOWBlankRWLayer uses HCS to create a temp test directory containing a
// read-write layer containing a disk that can be used as a containers scratch
// space. The VHD is created with VM group access
// TODO: This is wrong. Need to search the folders.
func CreateWCOWBlankRWLayer(t *testing.T, imageLayers []string) string {

	//	uvmFolder, err := LocateUVMFolder(imageLayers)
	//	if err != nil {
	//		t.Fatalf("failed to locate UVM folder from %+v: %s", imageLayers, err)
	//	}

	tempDir := CreateTempDir(t)
	if err := wclayer.CreateScratchLayer(tempDir, imageLayers); err != nil {
		t.Fatalf("Failed CreateScratchLayer: %s", err)
	}
	return tempDir
}

// CreateLCOWBlankRWLayer uses an LCOW utility VM to create a blank
// VHDX and format it ext4. If vmID is supplied, it grants access to the
// destination file. This can then be used as a scratch space for a container,
// or for a "service VM".
func CreateLCOWBlankRWLayer(t *testing.T, vmID string) string {
	if lcowGlobalSVM == nil {
		lcowGlobalSVM = CreateLCOWUVM(t, lcowGlobalSVMID)
		lcowCacheScratchFile = filepath.Join(CreateTempDir(t), "sandbox.vhdx")
	}
	tempDir := CreateTempDir(t)

	if err := lcow.CreateScratch(lcowGlobalSVM, filepath.Join(tempDir, "sandbox.vhdx"), lcow.DefaultScratchSizeGB, lcowCacheScratchFile, vmID); err != nil {
		t.Fatalf("failed to create EXT4 scratch for LCOW test cases: %s", err)
	}
	return tempDir
}
