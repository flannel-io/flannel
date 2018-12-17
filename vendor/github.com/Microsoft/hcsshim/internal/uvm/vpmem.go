package uvm

import (
	"fmt"

	"github.com/Microsoft/hcsshim/internal/guestrequest"
	"github.com/Microsoft/hcsshim/internal/requesttype"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/sirupsen/logrus"
)

// allocateVPMEM finds the next available VPMem slot. The lock MUST be held
// when calling this function.
func (uvm *UtilityVM) allocateVPMEM(hostPath string) (uint32, error) {
	for index, vi := range uvm.vpmemDevices {
		if vi.hostPath == "" {
			vi.hostPath = hostPath
			logrus.Debugf("uvm::allocateVPMEM %d %q", index, hostPath)
			return uint32(index), nil
		}
	}
	return 0, fmt.Errorf("no free VPMEM locations")
}

func (uvm *UtilityVM) deallocateVPMEM(deviceNumber uint32) error {
	uvm.m.Lock()
	defer uvm.m.Unlock()
	uvm.vpmemDevices[deviceNumber] = vpmemInfo{}
	return nil
}

// Lock must be held when calling this function
func (uvm *UtilityVM) findVPMEMDevice(findThisHostPath string) (uint32, string, error) {
	for deviceNumber, vi := range uvm.vpmemDevices {
		if vi.hostPath == findThisHostPath {
			logrus.Debugf("uvm::findVPMEMDeviceNumber %d %s", deviceNumber, findThisHostPath)
			return uint32(deviceNumber), vi.uvmPath, nil
		}
	}
	return 0, "", fmt.Errorf("%s is not attached to VPMEM", findThisHostPath)
}

// AddVPMEM adds a VPMEM disk to a utility VM at the next available location.
//
// Returns the location(0..MaxVPMEM-1) where the device is attached, and if exposed,
// the utility VM path which will be /tmp/p<location>//
func (uvm *UtilityVM) AddVPMEM(hostPath string, expose bool) (uint32, string, error) {
	if uvm.operatingSystem != "linux" {
		return 0, "", errNotSupported
	}

	logrus.Debugf("uvm::AddVPMEM id:%s hostPath:%s expose:%t", uvm.id, hostPath, expose)

	uvm.m.Lock()
	defer uvm.m.Unlock()

	var deviceNumber uint32
	var err error
	uvmPath := ""

	deviceNumber, uvmPath, err = uvm.findVPMEMDevice(hostPath)
	if err != nil {
		// Ensure the utility VM has access
		if err := wclayer.GrantVmAccess(uvm.ID(), hostPath); err != nil {
			return 0, "", err
		}

		// It doesn't exist, so we're going to allocate and hot-add it
		deviceNumber, err = uvm.allocateVPMEM(hostPath)
		if err != nil {
			return 0, "", err
		}

		modification := &hcsschema.ModifySettingRequest{
			RequestType: requesttype.Add,
			Settings: hcsschema.VirtualPMemDevice{
				HostPath:    hostPath,
				ReadOnly:    true,
				ImageFormat: "Vhd1",
			},
			ResourcePath: fmt.Sprintf("VirtualMachine/Devices/VirtualPMem/Devices/%d", deviceNumber),
		}

		if expose {
			uvmPath = fmt.Sprintf("/tmp/p%d", deviceNumber)
			modification.GuestRequest = guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeVPMemDevice,
				RequestType:  requesttype.Add,
				Settings: guestrequest.LCOWMappedVPMemDevice{
					DeviceNumber: deviceNumber,
					MountPath:    uvmPath,
				},
			}
		}

		if err := uvm.Modify(modification); err != nil {
			uvm.vpmemDevices[deviceNumber] = vpmemInfo{}
			return 0, "", fmt.Errorf("uvm::AddVPMEM: failed to modify utility VM configuration: %s", err)
		}

		uvm.vpmemDevices[deviceNumber] = vpmemInfo{
			hostPath: hostPath,
			refCount: 1,
			uvmPath:  uvmPath}
	} else {
		pmemi := vpmemInfo{
			hostPath: hostPath,
			refCount: uvm.vpmemDevices[deviceNumber].refCount + 1,
			uvmPath:  uvmPath}
		uvm.vpmemDevices[deviceNumber] = pmemi
	}
	logrus.Debugf("hcsshim::AddVPMEM id:%s Success %+v", uvm.id, uvm.vpmemDevices[deviceNumber])
	return deviceNumber, uvmPath, nil

}

// RemoveVPMEM removes a VPMEM disk from a utility VM. As an external API, it
// is "safe". Internal use can call removeVPMEM.
func (uvm *UtilityVM) RemoveVPMEM(hostPath string) error {
	if uvm.operatingSystem != "linux" {
		return errNotSupported
	}

	uvm.m.Lock()
	defer uvm.m.Unlock()

	// Make sure is actually attached
	deviceNumber, uvmPath, err := uvm.findVPMEMDevice(hostPath)
	if err != nil {
		return fmt.Errorf("cannot remove VPMEM %s as it is not attached to utility VM %s: %s", hostPath, uvm.id, err)
	}

	if err := uvm.removeVPMEM(hostPath, uvmPath, deviceNumber); err != nil {
		return fmt.Errorf("failed to remove VPMEM %s from utility VM %s: %s", hostPath, uvm.id, err)
	}
	return nil
}

// removeVPMEM is the internally callable "unsafe" version of RemoveVPMEM. The mutex
// MUST be held when calling this function.
func (uvm *UtilityVM) removeVPMEM(hostPath string, uvmPath string, deviceNumber uint32) error {
	logrus.Debugf("uvm::RemoveVPMEM id:%s hostPath:%s device:%d", uvm.id, hostPath, deviceNumber)

	if uvm.vpmemDevices[deviceNumber].refCount == 1 {
		modification := &hcsschema.ModifySettingRequest{
			RequestType:  requesttype.Remove,
			ResourcePath: fmt.Sprintf("VirtualMachine/Devices/VirtualPMem/Devices/%d", deviceNumber),
			GuestRequest: guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeVPMemDevice,
				RequestType:  requesttype.Remove,
				Settings: guestrequest.LCOWMappedVPMemDevice{
					DeviceNumber: deviceNumber,
					MountPath:    uvmPath,
				},
			},
		}

		if err := uvm.Modify(modification); err != nil {
			return err
		}
		uvm.vpmemDevices[deviceNumber] = vpmemInfo{}
		logrus.Debugf("uvm::RemoveVPMEM: Success id:%s hostPath:%s device:%d", uvm.id, hostPath, deviceNumber)
		return nil
	}
	uvm.vpmemDevices[deviceNumber].refCount--
	logrus.Debugf("uvm::RemoveVPMEM: Success id:%s hostPath:%s device:%d refCount:%d", uvm.id, hostPath, deviceNumber, uvm.vpmemDevices[deviceNumber].refCount)
	return nil

}
