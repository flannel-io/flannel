package uvm

// Terminate requests a utility VM terminate. If IsPending() on the error returned is true,
// it may not actually be shut down until Wait() succeeds.
func (uvm *UtilityVM) Terminate() error {
	return uvm.hcsSystem.Terminate()
}
