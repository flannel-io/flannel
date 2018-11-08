package testutilities

import (
	"testing"

	"github.com/Microsoft/hcsshim/internal/uvm"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// CreateWCOWUVM creates a WCOW utility VM. Returns the UtilityVM object; folder used as its scratch
func CreateWCOWUVM(t *testing.T, uvmLayers []string, id string, resources *specs.WindowsResources) (*uvm.UtilityVM, string) {
	scratchDir := CreateTempDir(t)

	opts := &uvm.UVMOptions{
		OperatingSystem: "windows",
		LayerFolders:    append(uvmLayers, scratchDir),
		Resources:       resources,
	}
	if id != "" {
		opts.ID = id
	}
	uvm, err := uvm.Create(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := uvm.Start(); err != nil {
		t.Fatal(err)
	}

	return uvm, scratchDir
}

// CreateLCOWUVM creates an LCOW utility VM.
func CreateLCOWUVM(t *testing.T, id string) *uvm.UtilityVM {
	opts := &uvm.UVMOptions{OperatingSystem: "linux"}
	if id != "" {
		opts.ID = id
	}
	uvm, err := uvm.Create(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := uvm.Start(); err != nil {
		t.Fatal(err)
	}
	return uvm
}
