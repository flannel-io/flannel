package uvm

import "github.com/Microsoft/hcsshim/internal/hcs"

func (uvm *UtilityVM) ComputeSystem() *hcs.System {
	return uvm.hcsSystem
}
