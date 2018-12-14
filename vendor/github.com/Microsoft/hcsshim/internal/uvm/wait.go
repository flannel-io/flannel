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
	uvm.waitForOutput()
	return err
}

func (uvm *UtilityVM) WaitExpectedError(expected error) error {
	err := uvm.hcsSystem.WaitExpectedError(expected)
	uvm.waitForOutput()
	return err
}
