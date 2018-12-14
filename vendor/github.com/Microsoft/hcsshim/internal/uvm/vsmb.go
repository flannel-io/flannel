package uvm

import (
	"fmt"
	"strconv"

	"github.com/Microsoft/hcsshim/internal/requesttype"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/sirupsen/logrus"
)

// findVSMBShare finds a share by `hostPath`. If not found returns `ErrNotAttached`.
func (uvm *UtilityVM) findVSMBShare(hostPath string) (*vsmbShare, error) {
	share, ok := uvm.vsmbShares[hostPath]
	if !ok {
		return nil, ErrNotAttached
	}
	return share, nil
}

func (share *vsmbShare) GuestPath() string {
	return `\\?\VMSMB\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\` + share.name
}

// AddVSMB adds a VSMB share to a Windows utility VM. Each VSMB share is ref-counted and
// only added if it isn't already. This is used for read-only layers, mapped directories
// to a container, and for mapped pipes.
func (uvm *UtilityVM) AddVSMB(hostPath string, guestRequest interface{}, options *hcsschema.VirtualSmbShareOptions) error {
	if uvm.operatingSystem != "windows" {
		return errNotSupported
	}

	logrus.Debugf("uvm::AddVSMB %s %+v %+v id:%s", hostPath, guestRequest, options, uvm.id)
	uvm.m.Lock()
	defer uvm.m.Unlock()
	share, err := uvm.findVSMBShare(hostPath)
	if err == ErrNotAttached {
		uvm.vsmbCounter++
		shareName := "s" + strconv.FormatUint(uvm.vsmbCounter, 16)

		modification := &hcsschema.ModifySettingRequest{
			RequestType: requesttype.Add,
			Settings: hcsschema.VirtualSmbShare{
				Name:    shareName,
				Options: options,
				Path:    hostPath,
			},
			ResourcePath: "VirtualMachine/Devices/VirtualSmb/Shares",
		}

		if err := uvm.Modify(modification); err != nil {
			return err
		}
		share = &vsmbShare{
			name:         shareName,
			guestRequest: guestRequest,
		}
		uvm.vsmbShares[hostPath] = share
	}
	share.refCount++
	logrus.Debugf("hcsshim::AddVSMB Success %s: refcount=%d %+v", hostPath, share.refCount, share)
	return nil
}

// RemoveVSMB removes a VSMB share from a utility VM. Each VSMB share is ref-counted
// and only actually removed when the ref-count drops to zero.
func (uvm *UtilityVM) RemoveVSMB(hostPath string) error {
	if uvm.operatingSystem != "windows" {
		return errNotSupported
	}
	logrus.Debugf("uvm::RemoveVSMB %s id:%s", hostPath, uvm.id)
	uvm.m.Lock()
	defer uvm.m.Unlock()
	share, err := uvm.findVSMBShare(hostPath)
	if err != nil {
		return fmt.Errorf("%s is not present as a VSMB share in %s, cannot remove", hostPath, uvm.id)
	}

	share.refCount--
	if share.refCount > 0 {
		logrus.Debugf("uvm::RemoveVSMB Success %s id:%s Ref-count now %d. It is still present in the utility VM", hostPath, uvm.id, share.refCount)
		return nil
	}
	logrus.Debugf("uvm::RemoveVSMB Zero ref-count, removing. %s id:%s", hostPath, uvm.id)
	modification := &hcsschema.ModifySettingRequest{
		RequestType:  requesttype.Remove,
		Settings:     hcsschema.VirtualSmbShare{Name: share.name},
		ResourcePath: "VirtualMachine/Devices/VirtualSmb/Shares",
	}
	if err := uvm.Modify(modification); err != nil {
		return fmt.Errorf("failed to remove vsmb share %s from %s: %+v: %s", hostPath, uvm.id, modification, err)
	}
	logrus.Debugf("uvm::RemoveVSMB Success %s id:%s successfully removed from utility VM", hostPath, uvm.id)
	delete(uvm.vsmbShares, hostPath)
	return nil
}

// GetVSMBUvmPath returns the guest path of a VSMB mount.
func (uvm *UtilityVM) GetVSMBUvmPath(hostPath string) (string, error) {
	if hostPath == "" {
		return "", fmt.Errorf("no hostPath passed to GetVSMBUvmPath")
	}
	uvm.m.Lock()
	defer uvm.m.Unlock()
	share, err := uvm.findVSMBShare(hostPath)
	if err != nil {
		return "", err
	}
	path := share.GuestPath()
	logrus.Debugf("uvm::GetVSMBUvmPath Success %s id:%s path:%s", hostPath, uvm.id, path)
	return path, nil
}
