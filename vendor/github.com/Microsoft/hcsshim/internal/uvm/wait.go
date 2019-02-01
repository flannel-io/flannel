package uvm

import (
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/sirupsen/logrus"
)

func (uvm *UtilityVM) waitForOutput() {
	logrus.WithField(logfields.UVMID, uvm.ID()).
		Debug("UVM exited, waiting for output processing to complete")
	if uvm.outputProcessingDone != nil {
		<-uvm.outputProcessingDone
	}
}

// Waits synchronously waits for a utility VM to terminate.
func (uvm *UtilityVM) Wait() error {
	err := uvm.hcsSystem.Wait()

	// outputProcessingCancel will only cancel waiting for the vsockexec
	// connection, it won't stop output processing once the connection is
	// established.
	if uvm.outputProcessingCancel != nil {
		uvm.outputProcessingCancel()
	}
	uvm.waitForOutput()

	return err
}

// WaitExpectedError synchronously waits for a utility VM to terminate. If the
// UVM terminates successfully, or if the given error is encountered internally
// during the wait, this function returns nil.
func (uvm *UtilityVM) WaitExpectedError(expected error) error {
	err := uvm.hcsSystem.WaitExpectedError(expected)

	// outputProcessingCancel will only cancel waiting for the vsockexec
	// connection, it won't stop output processing once the connection is
	// established.
	if uvm.outputProcessingCancel != nil {
		uvm.outputProcessingCancel()
	}
	uvm.waitForOutput()

	return err
}
