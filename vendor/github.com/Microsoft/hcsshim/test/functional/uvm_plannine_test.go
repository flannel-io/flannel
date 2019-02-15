// +build functional uvmp9

// This file isn't called uvm_plan9_test.go as go test skips when a number is in it... go figure (pun intended)

package functional

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/test/functional/utilities"
)

// TestPlan9 tests adding/removing Plan9 shares to/from a v2 Linux utility VM
// TODO: This is very basic. Need multiple shares and so-on. Can be iterated on later.
func TestPlan9(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)

	uvm := testutilities.CreateLCOWUVM(t, t.Name())
	defer uvm.Close()

	dir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(dir)
	var iterations uint32 = 64
	for i := 0; i < int(iterations); i++ {
		if err := uvm.AddPlan9(dir, fmt.Sprintf("/tmp/%s", filepath.Base(dir)), false); err != nil {
			t.Fatalf("AddPlan9 failed: %s", err)
		}
	}

	// Remove them all
	for i := 0; i < int(iterations); i++ {
		if err := uvm.RemovePlan9(dir); err != nil {
			t.Fatalf("RemovePlan9 failed: %s", err)
		}
	}
}
