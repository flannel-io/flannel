package uvm

import "fmt"

const (
	// MaxVPMEMCount is the maximum number of VPMem devices that may be added to an LCOW
	// utility VM
	MaxVPMEMCount = 128

	// DefaultVPMEMCount is the default number of VPMem devices that may be added to an LCOW
	// utility VM if the create request doesn't specify how many.
	DefaultVPMEMCount = 64

	// DefaultVPMemSizeBytes is the default size of a VPMem device if the create request
	// doesn't specify.
	DefaultVPMemSizeBytes = 4 * 1024 * 1024 * 1024 // 4GB
)

var errNotSupported = fmt.Errorf("not supported")
