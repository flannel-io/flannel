// +build windows

package hcsoci

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/Microsoft/hcsshim/internal/guestrequest"
	"github.com/Microsoft/hcsshim/internal/ospath"
	"github.com/Microsoft/hcsshim/internal/requesttype"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type lcowLayerEntry struct {
	hostPath string
	uvmPath  string
	scsi     bool
}

const scratchPath = "scratch"

// mountContainerLayers is a helper for clients to hide all the complexity of layer mounting
// Layer folder are in order: base, [rolayer1..rolayern,] scratch
//
// v1/v2: Argon WCOW: Returns the mount path on the host as a volume GUID.
// v1:    Xenon WCOW: Done internally in HCS, so no point calling doing anything here.
// v2:    Xenon WCOW: Returns a CombinedLayersV2 structure where ContainerRootPath is a folder
//                    inside the utility VM which is a GUID mapping of the scratch folder. Each
//                    of the layers are the VSMB locations where the read-only layers are mounted.
//
func MountContainerLayers(layerFolders []string, guestRoot string, uvm *uvm.UtilityVM) (interface{}, error) {
	logrus.Debugln("hcsshim::mountContainerLayers", layerFolders)

	if uvm == nil {
		if len(layerFolders) < 2 {
			return nil, fmt.Errorf("need at least two layers - base and scratch")
		}
		path := layerFolders[len(layerFolders)-1]
		rest := layerFolders[:len(layerFolders)-1]
		logrus.Debugln("hcsshim::mountContainerLayers ActivateLayer", path)
		if err := wclayer.ActivateLayer(path); err != nil {
			return nil, err
		}
		logrus.Debugln("hcsshim::mountContainerLayers Preparelayer", path, rest)
		if err := wclayer.PrepareLayer(path, rest); err != nil {
			if err2 := wclayer.DeactivateLayer(path); err2 != nil {
				logrus.Warnf("Failed to Deactivate %s: %s", path, err)
			}
			return nil, err
		}

		mountPath, err := wclayer.GetLayerMountPath(path)
		if err != nil {
			if err := wclayer.UnprepareLayer(path); err != nil {
				logrus.Warnf("Failed to Unprepare %s: %s", path, err)
			}
			if err2 := wclayer.DeactivateLayer(path); err2 != nil {
				logrus.Warnf("Failed to Deactivate %s: %s", path, err)
			}
			return nil, err
		}
		return mountPath, nil
	}

	// V2 UVM
	logrus.Debugf("hcsshim::mountContainerLayers Is a %s V2 UVM", uvm.OS())

	// 	Add each read-only layers. For Windows, this is a VSMB share with the ResourceUri ending in
	// a GUID based on the folder path. For Linux, this is a VPMEM device, except where is over the
	// max size supported, where we put it on SCSI instead.
	//
	//  Each layer is ref-counted so that multiple containers in the same utility VM can share them.
	var wcowLayersAdded []string
	var lcowlayersAdded []lcowLayerEntry
	attachedSCSIHostPath := ""

	for _, layerPath := range layerFolders[:len(layerFolders)-1] {
		var err error
		if uvm.OS() == "windows" {
			options := &hcsschema.VirtualSmbShareOptions{
				ReadOnly:            true,
				PseudoOplocks:       true,
				TakeBackupPrivilege: true,
				CacheIo:             true,
				ShareRead:           true,
			}
			err = uvm.AddVSMB(layerPath, "", options)
			if err == nil {
				wcowLayersAdded = append(wcowLayersAdded, layerPath)
			}
		} else {
			uvmPath := ""
			hostPath := filepath.Join(layerPath, "layer.vhd")

			var fi os.FileInfo
			fi, err = os.Stat(hostPath)
			if err == nil && uint64(fi.Size()) > uvm.PMemMaxSizeBytes() {
				// Too big for PMEM. Add on SCSI instead (at /tmp/S<C>/<L>).
				var (
					controller int
					lun        int32
				)
				controller, lun, err = uvm.AddSCSILayer(hostPath)
				if err == nil {
					lcowlayersAdded = append(lcowlayersAdded,
						lcowLayerEntry{
							hostPath: hostPath,
							uvmPath:  fmt.Sprintf("/tmp/S%d/%d", controller, lun),
							scsi:     true,
						})
				}
			} else {
				_, uvmPath, err = uvm.AddVPMEM(hostPath, true) // UVM path is calculated. Will be /tmp/vN/
				if err == nil {
					lcowlayersAdded = append(lcowlayersAdded,
						lcowLayerEntry{
							hostPath: hostPath,
							uvmPath:  uvmPath,
						})
				}
			}
		}
		if err != nil {
			cleanupOnMountFailure(uvm, wcowLayersAdded, lcowlayersAdded, attachedSCSIHostPath)
			return nil, err
		}
	}

	// Add the scratch at an unused SCSI location. The container path inside the
	// utility VM will be C:\<ID>.
	hostPath := filepath.Join(layerFolders[len(layerFolders)-1], "sandbox.vhdx")

	// BUGBUG Rename guestRoot better.
	containerScratchPathInUVM := ospath.Join(uvm.OS(), guestRoot, scratchPath)
	_, _, err := uvm.AddSCSI(hostPath, containerScratchPathInUVM, false)
	if err != nil {
		cleanupOnMountFailure(uvm, wcowLayersAdded, lcowlayersAdded, attachedSCSIHostPath)
		return nil, err
	}
	attachedSCSIHostPath = hostPath

	if uvm.OS() == "windows" {
		// 	Load the filter at the C:\s<ID> location calculated above. We pass into this request each of the
		// 	read-only layer folders.
		layers, err := computeV2Layers(uvm, wcowLayersAdded)
		if err != nil {
			cleanupOnMountFailure(uvm, wcowLayersAdded, lcowlayersAdded, attachedSCSIHostPath)
			return nil, err
		}
		guestRequest := guestrequest.CombinedLayers{
			ContainerRootPath: containerScratchPathInUVM,
			Layers:            layers,
		}
		combinedLayersModification := &hcsschema.ModifySettingRequest{
			GuestRequest: guestrequest.GuestRequest{
				Settings:     guestRequest,
				ResourceType: guestrequest.ResourceTypeCombinedLayers,
				RequestType:  requesttype.Add,
			},
		}
		if err := uvm.Modify(combinedLayersModification); err != nil {
			cleanupOnMountFailure(uvm, wcowLayersAdded, lcowlayersAdded, attachedSCSIHostPath)
			return nil, err
		}
		logrus.Debugln("hcsshim::mountContainerLayers Succeeded")
		return guestRequest, nil
	}

	// This is the LCOW layout inside the utilityVM. NNN is the container "number"
	// which increments for each container created in a utility VM.
	//
	// /run/gcs/c/NNN/config.json
	// /run/gcs/c/NNN/rootfs
	// /run/gcs/c/NNN/scratch/upper
	// /run/gcs/c/NNN/scratch/work
	//
	// /dev/sda on /tmp/scratch type ext4 (rw,relatime,block_validity,delalloc,barrier,user_xattr,acl)
	// /dev/pmem0 on /tmp/v0 type ext4 (ro,relatime,block_validity,delalloc,norecovery,barrier,dax,user_xattr,acl)
	// /dev/sdb on /run/gcs/c/NNN/scratch type ext4 (rw,relatime,block_validity,delalloc,barrier,user_xattr,acl)
	// overlay on /run/gcs/c/NNN/rootfs type overlay (rw,relatime,lowerdir=/tmp/v0,upperdir=/run/gcs/c/NNN/scratch/upper,workdir=/run/gcs/c/NNN/scratch/work)
	//
	// Where /dev/sda      is the scratch for utility VM itself
	//       /dev/pmemX    are read-only layers for containers
	//       /dev/sd(b...) are scratch spaces for each container

	layers := []hcsschema.Layer{}
	for _, l := range lcowlayersAdded {
		layers = append(layers, hcsschema.Layer{Path: l.uvmPath})
	}
	guestRequest := guestrequest.CombinedLayers{
		ContainerRootPath: path.Join(guestRoot, rootfsPath),
		Layers:            layers,
		ScratchPath:       containerScratchPathInUVM,
	}
	combinedLayersModification := &hcsschema.ModifySettingRequest{
		GuestRequest: guestrequest.GuestRequest{
			ResourceType: guestrequest.ResourceTypeCombinedLayers,
			RequestType:  requesttype.Add,
			Settings:     guestRequest,
		},
	}
	if err := uvm.Modify(combinedLayersModification); err != nil {
		cleanupOnMountFailure(uvm, wcowLayersAdded, lcowlayersAdded, attachedSCSIHostPath)
		return nil, err
	}
	logrus.Debugln("hcsshim::mountContainerLayers Succeeded")
	return guestRequest, nil

}

// UnmountOperation is used when calling Unmount() to determine what type of unmount is
// required. In V1 schema, this must be unmountOperationAll. In V2, client can
// be more optimal and only unmount what they need which can be a minor performance
// improvement (eg if you know only one container is running in a utility VM, and
// the UVM is about to be torn down, there's no need to unmount the VSMB shares,
// just SCSI to have a consistent file system).
type UnmountOperation uint

const (
	UnmountOperationSCSI  UnmountOperation = 0x01
	UnmountOperationVSMB                   = 0x02
	UnmountOperationVPMEM                  = 0x04
	UnmountOperationAll                    = UnmountOperationSCSI | UnmountOperationVSMB | UnmountOperationVPMEM
)

// UnmountContainerLayers is a helper for clients to hide all the complexity of layer unmounting
func UnmountContainerLayers(layerFolders []string, guestRoot string, uvm *uvm.UtilityVM, op UnmountOperation) error {
	logrus.Debugln("hcsshim::unmountContainerLayers", layerFolders)
	if uvm == nil {
		// Must be an argon - folders are mounted on the host
		if op != UnmountOperationAll {
			return fmt.Errorf("only operation supported for host-mounted folders is unmountOperationAll")
		}
		if len(layerFolders) < 1 {
			return fmt.Errorf("need at least one layer for Unmount")
		}
		path := layerFolders[len(layerFolders)-1]
		logrus.Debugln("hcsshim::Unmount UnprepareLayer", path)
		if err := wclayer.UnprepareLayer(path); err != nil {
			return err
		}
		// TODO Should we try this anyway?
		logrus.Debugln("hcsshim::unmountContainerLayers DeactivateLayer", path)
		return wclayer.DeactivateLayer(path)
	}

	// V2 Xenon

	// Base+Scratch as a minimum. This is different to v1 which only requires the scratch
	if len(layerFolders) < 2 {
		return fmt.Errorf("at least two layers are required for unmount")
	}

	var retError error

	// Unload the storage filter followed by the SCSI scratch
	if (op & UnmountOperationSCSI) == UnmountOperationSCSI {
		containerScratchPathInUVM := ospath.Join(uvm.OS(), guestRoot, scratchPath)
		logrus.Debugf("hcsshim::unmountContainerLayers CombinedLayers %s", containerScratchPathInUVM)
		combinedLayersModification := &hcsschema.ModifySettingRequest{
			GuestRequest: guestrequest.GuestRequest{
				ResourceType: guestrequest.ResourceTypeCombinedLayers,
				RequestType:  requesttype.Remove,
				Settings:     guestrequest.CombinedLayers{ContainerRootPath: containerScratchPathInUVM},
			},
		}
		if err := uvm.Modify(combinedLayersModification); err != nil {
			logrus.Errorf(err.Error())
		}

		// Hot remove the scratch from the SCSI controller
		hostScratchFile := filepath.Join(layerFolders[len(layerFolders)-1], "sandbox.vhdx")
		logrus.Debugf("hcsshim::unmountContainerLayers SCSI %s %s", containerScratchPathInUVM, hostScratchFile)
		if err := uvm.RemoveSCSI(hostScratchFile); err != nil {
			e := fmt.Errorf("failed to remove SCSI %s: %s", hostScratchFile, err)
			logrus.Debugln(e)
			if retError == nil {
				retError = e
			} else {
				retError = errors.Wrapf(retError, e.Error())
			}
		}
	}

	// Remove each of the read-only layers from VSMB. These's are ref-counted and
	// only removed once the count drops to zero. This allows multiple containers
	// to share layers.
	if uvm.OS() == "windows" && len(layerFolders) > 1 && (op&UnmountOperationVSMB) == UnmountOperationVSMB {
		for _, layerPath := range layerFolders[:len(layerFolders)-1] {
			if e := uvm.RemoveVSMB(layerPath); e != nil {
				logrus.Debugln(e)
				if retError == nil {
					retError = e
				} else {
					retError = errors.Wrapf(retError, e.Error())
				}
			}
		}
	}

	// Remove each of the read-only layers from VPMEM (or SCSI). These's are ref-counted
	// and only removed once the count drops to zero. This allows multiple containers to
	// share layers. Note that SCSI is used on large layers.
	if uvm.OS() == "linux" && len(layerFolders) > 1 && (op&UnmountOperationVPMEM) == UnmountOperationVPMEM {
		for _, layerPath := range layerFolders[:len(layerFolders)-1] {
			hostPath := filepath.Join(layerPath, "layer.vhd")
			if fi, err := os.Stat(hostPath); err != nil {
				var e error
				if uint64(fi.Size()) > uvm.PMemMaxSizeBytes() {
					e = uvm.RemoveSCSI(hostPath)
				} else {
					e = uvm.RemoveVPMEM(hostPath)
				}
				if e != nil {
					logrus.Debugln(e)
					if retError == nil {
						retError = e
					} else {
						retError = errors.Wrapf(retError, e.Error())
					}
				}
			}
		}
	}

	// TODO (possibly) Consider deleting the container directory in the utility VM

	return retError
}

func cleanupOnMountFailure(uvm *uvm.UtilityVM, wcowLayers []string, lcowLayers []lcowLayerEntry, scratchHostPath string) {
	for _, wl := range wcowLayers {
		if err := uvm.RemoveVSMB(wl); err != nil {
			logrus.Warnf("Possibly leaked vsmbshare on error removal path: %s", err)
		}
	}
	for _, ll := range lcowLayers {
		if ll.scsi {
			if err := uvm.RemoveSCSI(ll.hostPath); err != nil {
				logrus.Warnf("Possibly leaked SCSI on error removal path: %s", err)
			}
		} else if err := uvm.RemoveVPMEM(ll.hostPath); err != nil {
			logrus.Warnf("Possibly leaked vpmemdevice on error removal path: %s", err)
		}
	}
	if scratchHostPath != "" {
		if err := uvm.RemoveSCSI(scratchHostPath); err != nil {
			logrus.Warnf("Possibly leaked SCSI disk on error removal path: %s", err)
		}
	}
}

func computeV2Layers(vm *uvm.UtilityVM, paths []string) (layers []hcsschema.Layer, err error) {
	for _, path := range paths {
		uvmPath, err := vm.GetVSMBUvmPath(path)
		if err != nil {
			return nil, err
		}
		layerID, err := wclayer.LayerID(path)
		if err != nil {
			return nil, err
		}
		layers = append(layers, hcsschema.Layer{Id: layerID.String(), Path: uvmPath})
	}
	return layers, nil
}
