// +build functional uvmscratch

package functional

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Microsoft/hcsshim/internal/lcow"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/test/functional/utilities"
)

func TestScratchCreateLCOW(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	tempDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(tempDir)

	firstUVM := testutilities.CreateLCOWUVM(t, "TestCreateLCOWScratch")
	defer firstUVM.Close()

	cacheFile := filepath.Join(tempDir, "cache.vhdx")
	destOne := filepath.Join(tempDir, "destone.vhdx")
	destTwo := filepath.Join(tempDir, "desttwo.vhdx")

	if err := lcow.CreateScratch(firstUVM, destOne, lcow.DefaultScratchSizeGB, cacheFile, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(destOne); err != nil {
		t.Fatalf("destone wasn't created!")
	}
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cacheFile wasn't created!")
	}

	targetUVM := testutilities.CreateLCOWUVM(t, "TestCreateLCOWScratch_target")
	defer targetUVM.Close()

	// A non-cached create
	if err := lcow.CreateScratch(firstUVM, destTwo, lcow.DefaultScratchSizeGB, cacheFile, targetUVM.ID()); err != nil {
		t.Fatal(err)
	}

	// Make sure it can be added (verifies it has access correctly)
	c, l, err := targetUVM.AddSCSI(destTwo, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if c != 0 && l != 0 {
		t.Fatal(err)
	}
	// TODO Could consider giving it a host path and verifying it's contents somehow
}

// TODO This is old test which should go here.
//// createLCOWTempDirWithSandbox uses an LCOW utility VM to create a blank
//// VHDX and format it ext4.
//func TestCreateLCOWScratch(t *testing.T) {
//	t.Skip("for now")
//	cacheDir := createTempDir(t)
//	cacheFile := filepath.Join(cacheDir, "cache.vhdx")
//	uvm, err := CreateContainer(&CreateOptions{Spec: getDefaultLinuxSpec(t)})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	defer uvm.Terminate()
//	if err := uvm.Start(); err != nil {
//		t.Fatalf("Failed to start service container: %s", err)
//	}

//	// 1: Default size, cache doesn't exist, but no UVM passed. Cannot be created
//	err = CreateLCOWScratch(nil, filepath.Join(cacheDir, "default.vhdx"), lcow.DefaultScratchSizeGB, cacheFile)
//	if err == nil {
//		t.Fatalf("expected an error creating LCOW scratch")
//	}
//	if err.Error() != "cannot create scratch disk as cache is not present and no utility VM supplied" {
//		t.Fatalf("Not expecting error %s", err)
//	}

//	// 2: Default size, no cache supplied and no UVM
//	err = CreateLCOWScratch(nil, filepath.Join(cacheDir, "default.vhdx"), lcow.DefaultScratchSizeGB, "")
//	if err == nil {
//		t.Fatalf("expected an error creating LCOW scratch")
//	}
//	if err.Error() != "cannot create scratch disk as cache is not present and no utility VM supplied" {
//		t.Fatalf("Not expecting error %s", err)
//	}

//	// 3: Default size. This should work and the cache should be created.
//	err = CreateLCOWScratch(uvm, filepath.Join(cacheDir, "default.vhdx"), lcow.DefaultScratchSizeGB, cacheFile)
//	if err != nil {
//		t.Fatalf("should succeed creating default size cache file: %s", err)
//	}
//	if _, err = os.Stat(cacheFile); err != nil {
//		t.Fatalf("failed to stat cache file after created: %s", err)
//	}
//	if _, err = os.Stat(filepath.Join(cacheDir, "default.vhdx")); err != nil {
//		t.Fatalf("failed to stat default.vhdx after created: %s", err)
//	}

//	// 4: Non-defaultsize. This should work and the cache should be created.
//	err = CreateLCOWScratch(uvm, filepath.Join(cacheDir, "nondefault.vhdx"), lcow.DefaultScratchSizeGB+1, cacheFile)
//	if err != nil {
//		t.Fatalf("should succeed creating default size cache file: %s", err)
//	}
//	if _, err = os.Stat(cacheFile); err != nil {
//		t.Fatalf("failed to stat cache file after created: %s", err)
//	}
//	if _, err = os.Stat(filepath.Join(cacheDir, "nondefault.vhdx")); err != nil {
//		t.Fatalf("failed to stat default.vhdx after created: %s", err)
//	}

//}
