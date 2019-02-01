// +build functional uvmvpmem

package functional

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Microsoft/hcsshim/internal/copyfile"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/test/functional/utilities"
	"github.com/sirupsen/logrus"
)

// TestVPMEM tests adding/removing VPMem Read-Only layers from a v2 Linux utility VM
func TestVPMEM(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	alpineLayers := testutilities.LayerFolders(t, "alpine")

	u := testutilities.CreateLCOWUVM(t, t.Name())
	defer u.Close()

	var iterations uint32 = uvm.MaxVPMEMCount

	// Use layer.vhd from the alpine image as something to add
	tempDir := testutilities.CreateTempDir(t)
	if err := copyfile.CopyFile(filepath.Join(alpineLayers[0], "layer.vhd"), filepath.Join(tempDir, "layer.vhd"), true); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	for i := 0; i < int(iterations); i++ {
		deviceNumber, uvmPath, err := u.AddVPMEM(filepath.Join(tempDir, "layer.vhd"), true)
		if err != nil {
			t.Fatalf("AddVPMEM failed: %s", err)
		}
		logrus.Debugf("exposed as %s on %d", uvmPath, deviceNumber)
	}

	// Remove them all
	for i := 0; i < int(iterations); i++ {
		if err := u.RemoveVPMEM(filepath.Join(tempDir, "layer.vhd")); err != nil {
			t.Fatalf("RemoveVPMEM failed: %s", err)
		}
	}
}
