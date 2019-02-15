package uvm

import (
	"fmt"

	"github.com/Microsoft/hcsshim/internal/guestrequest"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/requesttype"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/sirupsen/logrus"
)

// AddPlan9 adds a Plan9 share to a utility VM. Each Plan9 share is ref-counted and
// only added if it isn't already.
func (uvm *UtilityVM) AddPlan9(hostPath string, uvmPath string, readOnly bool) error {
	logrus.WithFields(logrus.Fields{
		logfields.UVMID: uvm.id,
		"host-path":     hostPath,
		"uvm-path":      uvmPath,
		"readOnly":      readOnly,
	}).Debug("uvm::AddPlan9")

	if uvm.operatingSystem != "linux" {
		return errNotSupported
	}
	if uvmPath == "" {
		return fmt.Errorf("uvmPath must be passed to AddPlan9")
	}

	// TODO: JTERRY75 - These are marked private in the schema. For now use them
	// but when there are public variants we need to switch to them.
	const (
		shareFlagsReadOnly      int32 = 0x00000001
		shareFlagsLinuxMetadata int32 = 0x00000004
		shareFlagsCaseSensitive int32 = 0x00000008
	)

	// TODO: JTERRY75 - `shareFlagsCaseSensitive` only works if the Windows
	// `hostPath` supports case sensitivity. We need to detect this case before
	// forwarding this flag in all cases.
	flags := shareFlagsLinuxMetadata // | shareFlagsCaseSensitive
	if readOnly {
		flags |= shareFlagsReadOnly
	}

	uvm.m.Lock()
	defer uvm.m.Unlock()
	if uvm.plan9Shares == nil {
		uvm.plan9Shares = make(map[string]*plan9Info)
	}
	if _, ok := uvm.plan9Shares[hostPath]; !ok {
		uvm.plan9Counter++

		modification := &hcsschema.ModifySettingRequest{
			RequestType: requesttype.Add,
			Settings: hcsschema.Plan9Share{
				Name:  fmt.Sprintf("%d", uvm.plan9Counter),
				Path:  hostPath,
				Port:  int32(uvm.plan9Counter), // TODO: Temporary. Will all use a single port (9999)
				Flags: flags,
			},
			ResourcePath: fmt.Sprintf("VirtualMachine/Devices/Plan9/Shares"),
			GuestRequest: guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeMappedDirectory,
				RequestType:  requesttype.Add,
				Settings: guestrequest.LCOWMappedDirectory{
					MountPath: uvmPath,
					Port:      int32(uvm.plan9Counter), // TODO: Temporary. Will all use a single port (9999)
					ReadOnly:  readOnly,
				},
			},
		}

		if err := uvm.Modify(modification); err != nil {
			return err
		}
		uvm.plan9Shares[hostPath] = &plan9Info{
			refCount:  1,
			uvmPath:   uvmPath,
			idCounter: uvm.plan9Counter,
			port:      int32(uvm.plan9Counter), // TODO: Temporary. Will all use a single port (9999)
		}
	} else {
		uvm.plan9Shares[hostPath].refCount++
	}
	logrus.Debugf("hcsshim::AddPlan9 Success %s: refcount=%d %+v", hostPath, uvm.plan9Shares[hostPath].refCount, uvm.plan9Shares[hostPath])
	return nil
}

// RemovePlan9 removes a Plan9 share from a utility VM. Each Plan9 share is ref-counted
// and only actually removed when the ref-count drops to zero.
func (uvm *UtilityVM) RemovePlan9(hostPath string) error {
	if uvm.operatingSystem != "linux" {
		return errNotSupported
	}
	logrus.Debugf("uvm::RemovePlan9 %s id:%s", hostPath, uvm.id)
	uvm.m.Lock()
	defer uvm.m.Unlock()
	if _, ok := uvm.plan9Shares[hostPath]; !ok {
		return fmt.Errorf("%s is not present as a Plan9 share in %s, cannot remove", hostPath, uvm.id)
	}
	return uvm.removePlan9(hostPath, uvm.plan9Shares[hostPath].uvmPath)
}

// removePlan9 is the internally callable "unsafe" version of RemovePlan9. The mutex
// MUST be held when calling this function.
func (uvm *UtilityVM) removePlan9(hostPath, uvmPath string) error {
	uvm.plan9Shares[hostPath].refCount--
	if uvm.plan9Shares[hostPath].refCount > 0 {
		logrus.Debugf("uvm::RemovePlan9 Success %s id:%s Ref-count now %d. It is still present in the utility VM", hostPath, uvm.id, uvm.plan9Shares[hostPath].refCount)
		return nil
	}
	logrus.Debugf("uvm::RemovePlan9 Zero ref-count, removing. %s id:%s", hostPath, uvm.id)
	modification := &hcsschema.ModifySettingRequest{
		RequestType: requesttype.Remove,
		Settings: hcsschema.Plan9Share{
			Name: fmt.Sprintf("%d", uvm.plan9Shares[hostPath].idCounter),
			Port: uvm.plan9Shares[hostPath].port,
		},
		ResourcePath: fmt.Sprintf("VirtualMachine/Devices/Plan9/Shares"),
		GuestRequest: guestrequest.GuestRequest{
			ResourceType: guestrequest.ResourceTypeMappedDirectory,
			RequestType:  requesttype.Remove,
			Settings: guestrequest.LCOWMappedDirectory{
				MountPath: uvm.plan9Shares[hostPath].uvmPath,
				Port:      uvm.plan9Shares[hostPath].port,
			},
		},
	}
	if err := uvm.Modify(modification); err != nil {
		return fmt.Errorf("failed to remove plan9 share %s from %s: %+v: %s", hostPath, uvm.id, modification, err)
	}
	delete(uvm.plan9Shares, hostPath)
	logrus.Debugf("uvm::RemovePlan9 Success %s id:%s successfully removed from utility VM", hostPath, uvm.id)
	return nil
}
