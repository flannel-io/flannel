package uvm

// Modifies the compute system by sending a request to HCS
func (uvm *UtilityVM) Modify(hcsModificationDocument interface{}) error {
	return uvm.hcsSystem.Modify(hcsModificationDocument)
}
