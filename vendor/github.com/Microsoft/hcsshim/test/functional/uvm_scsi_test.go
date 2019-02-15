// +build functional uvmscsi

package functional

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/test/functional/utilities"
	"github.com/sirupsen/logrus"
)

// TestSCSIAddRemovev2LCOW validates adding and removing SCSI disks
// from a utility VM in both attach-only and with a container path. Also does
// negative testing so that a disk can't be attached twice.
func TestSCSIAddRemoveLCOW(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	u := testutilities.CreateLCOWUVM(t, t.Name())
	defer u.Close()

	testSCSIAddRemove(t, u, `/`, "linux", []string{})

}

// TestSCSIAddRemoveWCOW validates adding and removing SCSI disks
// from a utility VM in both attach-only and with a container path. Also does
// negative testing so that a disk can't be attached twice.
func TestSCSIAddRemoveWCOW(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	u, layers, uvmScratchDir := testutilities.CreateWCOWUVM(t, t.Name(), "microsoft/nanoserver")
	defer os.RemoveAll(uvmScratchDir)
	defer u.Close()

	testSCSIAddRemove(t, u, `c:\`, "windows", layers)
}

func testSCSIAddRemove(t *testing.T, u *uvm.UtilityVM, pathPrefix string, operatingSystem string, wcowImageLayerFolders []string) {
	numDisks := 63 // Windows: 63 as the UVM scratch is at 0:0
	if operatingSystem == "linux" {
		numDisks++ //
	}

	// Create a bunch of directories each containing sandbox.vhdx
	disks := make([]string, numDisks)
	for i := 0; i < numDisks; i++ {
		tempDir := ""
		if operatingSystem == "windows" {
			tempDir = testutilities.CreateWCOWBlankRWLayer(t, wcowImageLayerFolders)
		} else {
			tempDir = testutilities.CreateLCOWBlankRWLayer(t, u.ID())
		}
		defer os.RemoveAll(tempDir)
		disks[i] = filepath.Join(tempDir, `sandbox.vhdx`)
	}

	// Add each of the disks to the utility VM. Attach-only, no container path
	logrus.Debugln("First - adding in attach-only")
	for i := 0; i < numDisks; i++ {
		_, _, err := u.AddSCSI(disks[i], "", false)
		if err != nil {
			t.Fatalf("failed to add scsi disk %d %s: %s", i, disks[i], err)
		}
	}

	// Try to re-add. These should all fail.
	logrus.Debugln("Next - trying to re-add")
	for i := 0; i < numDisks; i++ {
		_, _, err := u.AddSCSI(disks[i], "", false)
		if err == nil {
			t.Fatalf("should not be able to re-add the same SCSI disk!")
		}
		if err != uvm.ErrAlreadyAttached {
			t.Fatalf("expecting %s, got %s", uvm.ErrAlreadyAttached, err)
		}
	}

	// Remove them all
	logrus.Debugln("Removing them all")
	for i := 0; i < numDisks; i++ {
		if err := u.RemoveSCSI(disks[i]); err != nil {
			t.Fatalf("expected success: %s", err)
		}
	}

	// Now re-add but providing a container path
	logrus.Debugln("Next - re-adding with a container path")
	for i := 0; i < numDisks; i++ {
		_, _, err := u.AddSCSI(disks[i], fmt.Sprintf(`%s%d`, pathPrefix, i), false)
		if err != nil {
			t.Fatalf("failed to add scsi disk %d %s: %s", i, disks[i], err)
		}
	}

	// Try to re-add. These should all fail.
	logrus.Debugln("Next - trying to re-add")
	for i := 0; i < numDisks; i++ {
		_, _, err := u.AddSCSI(disks[i], fmt.Sprintf(`%s%d`, pathPrefix, i), false)
		if err == nil {
			t.Fatalf("should not be able to re-add the same SCSI disk!")
		}
		if err != uvm.ErrAlreadyAttached {
			t.Fatalf("expecting %s, got %s", uvm.ErrAlreadyAttached, err)
		}
	}

	// Remove them all
	logrus.Debugln("Next - Removing them")
	for i := 0; i < numDisks; i++ {
		if err := u.RemoveSCSI(disks[i]); err != nil {
			t.Fatalf("expected success: %s", err)
		}
	}

	// TODO: Could extend to validate can't add a 64th disk (windows). 65th (linux).
}
