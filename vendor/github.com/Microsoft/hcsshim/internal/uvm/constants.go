package uvm

import "fmt"

const (
	// MaxVPMEM is the maximum number of VPMem devices that may be added to an LCOW
	// utility VM
	MaxVPMEM = 128

	// DefaultVPMEM is the default number of VPMem devices that may be added to an LCOW
	// utility VM if the create request doesn't specify how many.
	DefaultVPMEM = 64
)

var errNotSupported = fmt.Errorf("not supported")
