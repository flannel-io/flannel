package uvm

// This package describes the external interface for utility VMs.

import (
	"context"
	"net"
	"sync"

	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hns"
)

//                    | WCOW | LCOW
// Container scratch  | SCSI | SCSI
// Scratch space      | ---- | SCSI   // For file system utilities. /tmp/scratch
// Read-Only Layer    | VSMB | VPMEM
// Mapped Directory   | VSMB | PLAN9

// vsmbShare is an internal structure used for ref-counting VSMB shares mapped to a Windows utility VM.
type vsmbShare struct {
	refCount     uint32
	name         string
	guestRequest interface{}
}

// scsiInfo is an internal structure used for determining what is mapped to a utility VM.
// hostPath is required. uvmPath may be blank.
type scsiInfo struct {
	hostPath string
	uvmPath  string

	// While most VHDs attached to SCSI are scratch spaces, in the case of LCOW
	// when the size is over the size possible to attach to PMEM, we use SCSI for
	// read-only layers. As RO layers are shared, we perform ref-counting.
	isLayer  bool
	refCount uint32
}

// vpmemInfo is an internal structure used for determining VPMem devices mapped to
// a Linux utility VM.
type vpmemInfo struct {
	hostPath string
	uvmPath  string
	refCount uint32
}

// plan9Info is an internal structure used for ref-counting Plan9 shares mapped to a Linux utility VM.
type plan9Info struct {
	refCount  uint32
	idCounter uint64
	uvmPath   string
	port      int32 // Temporary. TODO Remove
}
type nicInfo struct {
	ID       guid.GUID
	Endpoint *hns.HNSEndpoint
}

type namespaceInfo struct {
	nics     []nicInfo
	refCount int
}

// UtilityVM is the object used by clients representing a utility VM
type UtilityVM struct {
	id              string      // Identifier for the utility VM (user supplied or generated)
	owner           string      // Owner for the utility VM (user supplied or generated)
	operatingSystem string      // "windows" or "linux"
	hcsSystem       *hcs.System // The handle to the compute system
	m               sync.Mutex  // Lock for adding/removing devices

	// containerCounter is the current number of containers that have been
	// created. This is never decremented in the life of the UVM.
	//
	// NOTE: All accesses to this MUST be done atomically.
	containerCounter uint64

	// VSMB shares that are mapped into a Windows UVM. These are used for read-only
	// layers and mapped directories
	vsmbShares  map[string]*vsmbShare
	vsmbCounter uint64 // Counter to generate a unique share name for each VSMB share.

	// VPMEM devices that are mapped into a Linux UVM. These are used for read-only layers, or for
	// booting from VHD.
	vpmemDevices      [MaxVPMEMCount]vpmemInfo // Limited by ACPI size.
	vpmemMaxCount     uint32                   // Actual number of VPMem devices
	vpmemMaxSizeBytes uint64                   // Actual size of VPMem devices

	// SCSI devices that are mapped into a Windows or Linux utility VM
	scsiLocations       [4][64]scsiInfo // Hyper-V supports 4 controllers, 64 slots per controller. Limited to 1 controller for now though.
	scsiControllerCount uint32          // Number of SCSI controllers in the utility VM

	// Plan9 are directories mapped into a Linux utility VM
	plan9Shares  map[string]*plan9Info
	plan9Counter uint64 // Each newly-added plan9 share has a counter used as its ID in the ResourceURI and for the name

	namespaces map[string]*namespaceInfo

	outputListener         net.Listener
	outputProcessingDone   chan struct{}
	outputHandler          OutputHandler
	outputProcessingCancel context.CancelFunc
}
