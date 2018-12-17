// +build windows

package hcsoci

// Contains functions relating to a LCOW container, as opposed to a utility VM

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim/internal/guestrequest"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const rootfsPath = "rootfs"
const mountPathPrefix = "m"

func allocateLinuxResources(coi *createOptionsInternal, resources *Resources) error {
	if coi.Spec.Root == nil {
		coi.Spec.Root = &specs.Root{}
	}
	if coi.Spec.Root.Path == "" {
		logrus.Debugln("hcsshim::allocateLinuxResources mounting storage")
		mcl, err := MountContainerLayers(coi.Spec.Windows.LayerFolders, resources.containerRootInUVM, coi.HostingSystem)
		if err != nil {
			return fmt.Errorf("failed to mount container storage: %s", err)
		}
		if coi.HostingSystem == nil {
			coi.Spec.Root.Path = mcl.(string) // Argon v1 or v2
		} else {
			coi.Spec.Root.Path = mcl.(guestrequest.CombinedLayers).ContainerRootPath // v2 Xenon LCOW
		}
		resources.layers = coi.Spec.Windows.LayerFolders
	} else {
		// This is the "Plan 9" root filesystem.
		// TODO: We need a test for this. Ask @jstarks how you can even lay this out on Windows.
		hostPath := coi.Spec.Root.Path
		uvmPathForContainersFileSystem := path.Join(resources.containerRootInUVM, rootfsPath)
		err := coi.HostingSystem.AddPlan9(hostPath, uvmPathForContainersFileSystem, coi.Spec.Root.Readonly)
		if err != nil {
			return fmt.Errorf("adding plan9 root: %s", err)
		}
		coi.Spec.Root.Path = uvmPathForContainersFileSystem
		resources.plan9Mounts = append(resources.plan9Mounts, hostPath)
	}

	for i, mount := range coi.Spec.Mounts {
		switch mount.Type {
		case "bind":
		case "physical-disk":
		case "virtual-disk":
		default:
			// Unknown mount type
			continue
		}
		if mount.Destination == "" || mount.Source == "" {
			return fmt.Errorf("invalid OCI spec - a mount must have both source and a destination: %+v", mount)
		}

		if coi.HostingSystem != nil {
			hostPath := mount.Source
			uvmPathForShare := path.Join(resources.containerRootInUVM, mountPathPrefix+strconv.Itoa(i))

			readOnly := false
			for _, o := range mount.Options {
				if strings.ToLower(o) == "ro" {
					readOnly = true
					break
				}
			}

			if mount.Type == "physical-disk" {
				logrus.Debugf("hcsshim::allocateLinuxResources Hot-adding SCSI physical disk for OCI mount %+v", mount)
				_, _, err := coi.HostingSystem.AddSCSIPhysicalDisk(hostPath, uvmPathForShare, readOnly)
				if err != nil {
					return fmt.Errorf("adding SCSI physical disk mount %+v: %s", mount, err)
				}
				resources.scsiMounts = append(resources.scsiMounts, hostPath)
				coi.Spec.Mounts[i].Type = "none"
			} else if mount.Type == "virtual-disk" {
				logrus.Debugf("hcsshim::allocateLinuxResources Hot-adding SCSI virtual disk for OCI mount %+v", mount)
				_, _, err := coi.HostingSystem.AddSCSI(hostPath, uvmPathForShare, readOnly)
				if err != nil {
					return fmt.Errorf("adding SCSI virtual disk mount %+v: %s", mount, err)
				}
				resources.scsiMounts = append(resources.scsiMounts, hostPath)
				coi.Spec.Mounts[i].Type = "none"
			} else {
				logrus.Debugf("hcsshim::allocateLinuxResources Hot-adding Plan9 for OCI mount %+v", mount)
				err := coi.HostingSystem.AddPlan9(hostPath, uvmPathForShare, readOnly)
				if err != nil {
					return fmt.Errorf("adding plan9 mount %+v: %s", mount, err)
				}
				resources.plan9Mounts = append(resources.plan9Mounts, hostPath)
			}
			coi.Spec.Mounts[i].Source = uvmPathForShare
		}
	}

	return nil
}
