package uvm

import (
	"testing"
)

// Unit tests for negative testing of input to uvm.Create()

func TestCreateBadOS(t *testing.T) {
	opts := &UVMOptions{
		OperatingSystem: "foobar",
	}
	_, err := Create(opts)
	if err == nil || (err != nil && err.Error() != `unsupported operating system "foobar"`) {
		t.Fatal(err)
	}
}

func TestCreateBadBootFilesPath(t *testing.T) {
	opts := &UVMOptions{
		OperatingSystem: "linux",
		BootFilesPath:   `c:\does\not\exist\I\hope`,
	}
	_, err := Create(opts)
	if err == nil || (err != nil && err.Error() != `kernel 'c:\does\not\exist\I\hope\kernel' not found`) {
		t.Fatal(err)
	}
}

func TestCreateWCOWBadLayerFolders(t *testing.T) {
	opts := &UVMOptions{
		OperatingSystem: "windows",
	}
	_, err := Create(opts)
	if err == nil || (err != nil && err.Error() != `at least 2 LayerFolders must be supplied`) {
		t.Fatal(err)
	}
}
