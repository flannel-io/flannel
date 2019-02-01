package uvm

import (
	"runtime"
)

// Options are the set of options passed to Create() to create a utility vm.
type Options struct {
	ID                      string // Identifier for the uvm. Defaults to generated GUID.
	Owner                   string // Specifies the owner. Defaults to executable name.
	AdditionHCSDocumentJSON string // Optional additional JSON to merge into the HCS document prior

	// MemorySizeInMB sets the UVM memory. If `0` will default to platform
	// default.
	MemorySizeInMB int32

	// Memory for UVM. Defaults to true. For physical backed memory, set to
	// false.
	AllowOvercommit bool

	// Memory for UVM. Defaults to false. For virtual memory with deferred
	// commit, set to true.
	EnableDeferredCommit bool

	// ProcessorCount sets the number of vCPU's. If `0` will default to platform
	// default.
	ProcessorCount int32
}

// ID returns the ID of the VM's compute system.
func (uvm *UtilityVM) ID() string {
	return uvm.hcsSystem.ID()
}

// OS returns the operating system of the utility VM.
func (uvm *UtilityVM) OS() string {
	return uvm.operatingSystem
}

// Close terminates and releases resources associated with the utility VM.
func (uvm *UtilityVM) Close() error {
	uvm.Terminate()

	// outputListener will only be nil for a Create -> Stop without a Start. In
	// this case we have no goroutine processing output so its safe to close the
	// channel here.
	if uvm.outputListener != nil {
		close(uvm.outputProcessingDone)
		uvm.outputListener.Close()
		uvm.outputListener = nil
	}
	err := uvm.hcsSystem.Close()
	uvm.hcsSystem = nil
	return err
}

func defaultProcessorCount() int32 {
	if runtime.NumCPU() == 1 {
		return 1
	}
	return 2
}
