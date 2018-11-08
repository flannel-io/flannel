package uvm

// Waits synchronously waits for a utility VM to terminate.
func (uvm *UtilityVM) Wait() error {
	return uvm.hcsSystem.Wait()
}
