package uvm

import (
	"fmt"

	"github.com/Microsoft/hcsshim/internal/guestrequest"
	"github.com/Microsoft/hcsshim/internal/requesttype"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/sirupsen/logrus"
)

var (
	ErrNoAvailableLocation = fmt.Errorf("no available location")
	ErrNotAttached         = fmt.Errorf("not attached")
	ErrAlreadyAttached     = fmt.Errorf("already attached")
	ErrNoSCSIControllers   = fmt.Errorf("no SCSI controllers configured for this utility VM")
	ErrNoUvmParameter      = fmt.Errorf("invalid parameters - uvm parameter missing")
	ErrTooManyAttachments  = fmt.Errorf("too many SCSI attachments")
)

// allocateSCSI finds the next available slot on the
// SCSI controllers associated with a utility VM to use.
func (uvm *UtilityVM) allocateSCSI(hostPath string, uvmPath string) (int, int32, error) {
	uvm.m.Lock()
	defer uvm.m.Unlock()
	for controller, luns := range uvm.scsiLocations {
		for lun, si := range luns {
			if si.hostPath == "" {
				uvm.scsiLocations[controller][lun].hostPath = hostPath
				uvm.scsiLocations[controller][lun].uvmPath = uvmPath
				logrus.Debugf("uvm::allocateSCSI %d:%d %q %q", controller, lun, hostPath, uvmPath)
				return controller, int32(lun), nil

			}
		}
	}
	return -1, -1, ErrNoAvailableLocation
}

func (uvm *UtilityVM) deallocateSCSI(controller int, lun int32) error {
	uvm.m.Lock()
	defer uvm.m.Unlock()
	logrus.Debugf("uvm::deallocateSCSI %d:%d %+v", controller, lun, uvm.scsiLocations[controller][lun])
	uvm.scsiLocations[controller][lun] = scsiInfo{}

	return nil
}

// Lock must be held when calling this function
func (uvm *UtilityVM) findSCSIAttachment(findThisHostPath string) (int, int32, string, error) {
	for controller, luns := range uvm.scsiLocations {
		for lun, si := range luns {
			if si.hostPath == findThisHostPath {
				logrus.Debugf("uvm::findSCSIAttachment %d:%d %+v", controller, lun, si)
				return controller, int32(lun), si.uvmPath, nil
			}
		}
	}
	return -1, -1, "", ErrNotAttached
}

// AddSCSI adds a SCSI disk to a utility VM at the next available location.
//
// We are in control of everything ourselves. Hence we have ref-
// counting and so-on tracking what SCSI locations are available or used.
//
// hostPath is required
// uvmPath is optional.
//
// Returns the controller ID (0..3) and LUN (0..63) where the disk is attached.
func (uvm *UtilityVM) AddSCSI(hostPath string, uvmPath string) (int, int32, error) {
	if uvm == nil {
		return -1, -1, ErrNoUvmParameter
	}
	logrus.Debugf("uvm::AddSCSI id:%s hostPath:%s uvmPath:%s", uvm.id, hostPath, uvmPath)

	if uvm.scsiControllerCount == 0 {
		return -1, -1, ErrNoSCSIControllers
	}

	uvm.m.Lock()
	if _, _, _, err := uvm.findSCSIAttachment(hostPath); err == nil {
		uvm.m.Unlock()
		return -1, -1, ErrAlreadyAttached
	}
	uvm.m.Unlock()

	controller, lun, err := uvm.allocateSCSI(hostPath, uvmPath)
	if err != nil {
		return -1, -1, err
	}

	// Note: Can remove this check post-RS5 if multiple controllers are supported
	if controller > 0 {
		return -1, -1, ErrTooManyAttachments
	}

	SCSIModification := &hcsschema.ModifySettingRequest{
		RequestType: requesttype.Add,
		Settings: hcsschema.Attachment{
			Path:  hostPath,
			Type_: "VirtualDisk",
		},
		ResourcePath: fmt.Sprintf("VirtualMachine/Devices/Scsi/%d/Attachments/%d", controller, lun),
	}

	if uvmPath != "" {
		if uvm.operatingSystem == "windows" {
			SCSIModification.GuestRequest = guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeMappedVirtualDisk,
				RequestType:  requesttype.Add,
				Settings: guestrequest.WCOWMappedVirtualDisk{
					ContainerPath: uvmPath,
					Lun:           lun,
				},
			}
		} else {
			SCSIModification.GuestRequest = guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeMappedVirtualDisk,
				RequestType:  requesttype.Add,
				Settings: guestrequest.LCOWMappedVirtualDisk{
					MountPath:  uvmPath,
					Lun:        uint8(lun),
					Controller: uint8(controller),
					ReadOnly:   false,
				},
			}
		}
	}

	if err := uvm.Modify(SCSIModification); err != nil {
		uvm.deallocateSCSI(controller, lun)
		return -1, -1, fmt.Errorf("uvm::AddSCSI: failed to modify utility VM configuration: %s", err)
	}
	logrus.Debugf("uvm::AddSCSI id:%s hostPath:%s added at %d:%d", uvm.id, hostPath, controller, lun)
	return controller, int32(lun), nil

}

// RemoveSCSI removes a SCSI disk from a utility VM. As an external API, it
// is "safe". Internal use can call removeSCSI.
func (uvm *UtilityVM) RemoveSCSI(hostPath string) error {
	uvm.m.Lock()
	defer uvm.m.Unlock()

	if uvm.scsiControllerCount == 0 {
		return ErrNoSCSIControllers
	}

	// Make sure is actually attached
	controller, lun, uvmPath, err := uvm.findSCSIAttachment(hostPath)
	if err != nil {
		return err
	}

	if err := uvm.removeSCSI(hostPath, uvmPath, controller, lun); err != nil {
		return fmt.Errorf("failed to remove SCSI disk %s from container %s: %s", hostPath, uvm.id, err)

	}
	return nil
}

// removeSCSI is the internally callable "unsafe" version of RemoveSCSI. The mutex
// MUST be held when calling this function.
func (uvm *UtilityVM) removeSCSI(hostPath string, uvmPath string, controller int, lun int32) error {
	logrus.Debugf("uvm::RemoveSCSI id:%s hostPath:%s", uvm.id, hostPath)
	scsiModification := &hcsschema.ModifySettingRequest{
		RequestType:  requesttype.Remove,
		ResourcePath: fmt.Sprintf("VirtualMachine/Devices/Scsi/%d/Attachments/%d", controller, lun),
	}

	// Include the GuestRequest so that the GCS ejects the disk cleanly if the disk was attached/mounted
	if uvmPath != "" {
		if uvm.operatingSystem == "windows" {
			scsiModification.GuestRequest = guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeMappedVirtualDisk,
				RequestType:  requesttype.Remove,
				Settings: guestrequest.WCOWMappedVirtualDisk{
					ContainerPath: uvmPath,
					Lun:           lun,
				},
			}
		} else {
			scsiModification.GuestRequest = guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeMappedVirtualDisk,
				RequestType:  requesttype.Remove,
				Settings: guestrequest.LCOWMappedVirtualDisk{
					MountPath:  uvmPath, // May be blank in attach-only
					Lun:        uint8(lun),
					Controller: uint8(controller),
				},
			}
		}
	}

	if err := uvm.Modify(scsiModification); err != nil {
		return err
	}
	uvm.scsiLocations[controller][lun] = scsiInfo{}
	logrus.Debugf("uvm::RemoveSCSI: Success %s removed from %s %d:%d", hostPath, uvm.id, controller, lun)
	return nil
}
