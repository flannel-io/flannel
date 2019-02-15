package uvm

import (
	"testing"
)

// Unit tests for negative testing of input to uvm.Create()

func TestCreateBadBootFilesPath(t *testing.T) {
	opts := NewDefaultOptionsLCOW(t.Name(), "")
	opts.BootFilesPath = `c:\does\not\exist\I\hope`

	_, err := CreateLCOW(opts)
	if err == nil || err.Error() != `kernel: 'c:\does\not\exist\I\hope\kernel' not found` {
		t.Fatal(err)
	}
}

func TestCreateWCOWBadLayerFolders(t *testing.T) {
	opts := NewDefaultOptionsWCOW(t.Name(), "")
	_, err := CreateWCOW(opts)
	if err == nil || (err != nil && err.Error() != `at least 2 LayerFolders must be supplied`) {
		t.Fatal(err)
	}
}
