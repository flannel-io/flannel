// +build windows

package hcsoci

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Microsoft/hcsshim/internal/schema1"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/internal/schemaversion"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/internal/uvmfolder"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/sirupsen/logrus"
)

// createWindowsContainerDocument creates a document suitable for calling HCS to create
// a container, both hosted and process isolated. It can create both v1 and v2
// schema, WCOW only. The containers storage should have been mounted already.
func createWindowsContainerDocument(coi *createOptionsInternal) (interface{}, error) {
	logrus.Debugf("hcsshim: CreateHCSContainerDocument")
	// TODO: Make this safe if exported so no null pointer dereferences.

	if coi.Spec == nil {
		return nil, fmt.Errorf("cannot create HCS container document - OCI spec is missing")
	}

	if coi.Spec.Windows == nil {
		return nil, fmt.Errorf("cannot create HCS container document - OCI spec Windows section is missing ")
	}

	v1 := &schema1.ContainerConfig{
		SystemType:              "Container",
		Name:                    coi.actualID,
		Owner:                   coi.actualOwner,
		HvPartition:             false,
		IgnoreFlushesDuringBoot: coi.Spec.Windows.IgnoreFlushesDuringBoot,
	}

	// IgnoreFlushesDuringBoot is a property of the SCSI attachment for the scratch. Set when it's hot-added to the utility VM
	// ID is a property on the create call in V2 rather than part of the schema.
	v2 := &hcsschema.ComputeSystem{
		Owner:                             coi.actualOwner,
		SchemaVersion:                     schemaversion.SchemaV21(),
		ShouldTerminateOnLastHandleClosed: true,
	}
	v2Container := &hcsschema.Container{Storage: &hcsschema.Storage{}}

	// TODO: Still want to revisit this.
	if coi.Spec.Windows.LayerFolders == nil || len(coi.Spec.Windows.LayerFolders) < 2 {
		return nil, fmt.Errorf("invalid spec - not enough layer folders supplied")
	}

	if coi.Spec.Hostname != "" {
		v1.HostName = coi.Spec.Hostname
		v2Container.GuestOs = &hcsschema.GuestOs{HostName: coi.Spec.Hostname}
	}

	if coi.Spec.Windows.Resources != nil {
		if coi.Spec.Windows.Resources.CPU != nil {
			if coi.Spec.Windows.Resources.CPU.Count != nil ||
				coi.Spec.Windows.Resources.CPU.Shares != nil ||
				coi.Spec.Windows.Resources.CPU.Maximum != nil {
				v2Container.Processor = &hcsschema.Processor{}
			}
			if coi.Spec.Windows.Resources.CPU.Count != nil {
				cpuCount := *coi.Spec.Windows.Resources.CPU.Count
				hostCPUCount := uint64(runtime.NumCPU())
				if cpuCount > hostCPUCount {
					logrus.Warnf("Changing requested CPUCount of %d to current number of processors, %d", cpuCount, hostCPUCount)
					cpuCount = hostCPUCount
				}
				v1.ProcessorCount = uint32(cpuCount)
				v2Container.Processor.Count = int32(cpuCount)
			}
			if coi.Spec.Windows.Resources.CPU.Shares != nil {
				v1.ProcessorWeight = uint64(*coi.Spec.Windows.Resources.CPU.Shares)
				v2Container.Processor.Weight = int32(v1.ProcessorWeight)
			}
			if coi.Spec.Windows.Resources.CPU.Maximum != nil {
				v1.ProcessorMaximum = int64(*coi.Spec.Windows.Resources.CPU.Maximum)
				v2Container.Processor.Maximum = int32(v1.ProcessorMaximum)
			}
		}
		if coi.Spec.Windows.Resources.Memory != nil {
			if coi.Spec.Windows.Resources.Memory.Limit != nil {
				v1.MemoryMaximumInMB = int64(*coi.Spec.Windows.Resources.Memory.Limit) / 1024 / 1024
				v2Container.Memory = &hcsschema.Memory{SizeInMB: int32(v1.MemoryMaximumInMB)}

			}
		}
		if coi.Spec.Windows.Resources.Storage != nil {
			if coi.Spec.Windows.Resources.Storage.Bps != nil || coi.Spec.Windows.Resources.Storage.Iops != nil {
				v2Container.Storage.QoS = &hcsschema.StorageQoS{}
			}
			if coi.Spec.Windows.Resources.Storage.Bps != nil {
				v1.StorageBandwidthMaximum = *coi.Spec.Windows.Resources.Storage.Bps
				v2Container.Storage.QoS.BandwidthMaximum = int32(v1.StorageBandwidthMaximum)
			}
			if coi.Spec.Windows.Resources.Storage.Iops != nil {
				v1.StorageIOPSMaximum = *coi.Spec.Windows.Resources.Storage.Iops
				v2Container.Storage.QoS.IopsMaximum = int32(*coi.Spec.Windows.Resources.Storage.Iops)
			}
		}
	}

	// TODO V2 networking. Only partial at the moment. v2.Container.Networking.Namespace specifically
	if coi.Spec.Windows.Network != nil {
		v2Container.Networking = &hcsschema.Networking{}

		v1.EndpointList = coi.Spec.Windows.Network.EndpointList
		v2Container.Networking.Namespace = coi.actualNetworkNamespace

		v1.AllowUnqualifiedDNSQuery = coi.Spec.Windows.Network.AllowUnqualifiedDNSQuery
		v2Container.Networking.AllowUnqualifiedDnsQuery = v1.AllowUnqualifiedDNSQuery

		if coi.Spec.Windows.Network.DNSSearchList != nil {
			v1.DNSSearchList = strings.Join(coi.Spec.Windows.Network.DNSSearchList, ",")
			v2Container.Networking.DnsSearchList = v1.DNSSearchList
		}

		v1.NetworkSharedContainerName = coi.Spec.Windows.Network.NetworkSharedContainerName
		v2Container.Networking.NetworkSharedContainerName = v1.NetworkSharedContainerName
	}

	//	// TODO V2 Credentials not in the schema yet.
	if cs, ok := coi.Spec.Windows.CredentialSpec.(string); ok {
		v1.Credentials = cs
	}

	if coi.Spec.Root == nil {
		return nil, fmt.Errorf("spec is invalid - root isn't populated")
	}

	if coi.Spec.Root.Readonly {
		return nil, fmt.Errorf(`invalid container spec - readonly is not supported for Windows containers`)
	}

	// Strip off the top-most RW/scratch layer as that's passed in separately to HCS for v1
	v1.LayerFolderPath = coi.Spec.Windows.LayerFolders[len(coi.Spec.Windows.LayerFolders)-1]

	if (schemaversion.IsV21(coi.actualSchemaVersion) && coi.HostingSystem == nil) ||
		(schemaversion.IsV10(coi.actualSchemaVersion) && coi.Spec.Windows.HyperV == nil) {
		// Argon v1 or v2.
		const volumeGUIDRegex = `^\\\\\?\\(Volume)\{{0,1}[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}(\}){0,1}\}(|\\)$`
		if matched, err := regexp.MatchString(volumeGUIDRegex, coi.Spec.Root.Path); !matched || err != nil {
			return nil, fmt.Errorf(`invalid container spec - Root.Path '%s' must be a volume GUID path in the format '\\?\Volume{GUID}\'`, coi.Spec.Root.Path)
		}
		if coi.Spec.Root.Path[len(coi.Spec.Root.Path)-1] != '\\' {
			coi.Spec.Root.Path += `\` // Be nice to clients and make sure well-formed for back-compat
		}
		v1.VolumePath = coi.Spec.Root.Path[:len(coi.Spec.Root.Path)-1] // Strip the trailing backslash. Required for v1.
		v2Container.Storage.Path = coi.Spec.Root.Path
	} else {
		// A hosting system was supplied, implying v2 Xenon; OR a v1 Xenon.
		if schemaversion.IsV10(coi.actualSchemaVersion) {
			// V1 Xenon
			v1.HvPartition = true
			if coi.Spec == nil || coi.Spec.Windows == nil || coi.Spec.Windows.HyperV == nil { // Be resilient to nil de-reference
				return nil, fmt.Errorf(`invalid container spec - Spec.Windows.HyperV is nil`)
			}
			if coi.Spec.Windows.HyperV.UtilityVMPath != "" {
				// Client-supplied utility VM path
				v1.HvRuntime = &schema1.HvRuntime{ImagePath: coi.Spec.Windows.HyperV.UtilityVMPath}
			} else {
				// Client was lazy. Let's locate it from the layer folders instead.
				uvmImagePath, err := uvmfolder.LocateUVMFolder(coi.Spec.Windows.LayerFolders)
				if err != nil {
					return nil, err
				}
				v1.HvRuntime = &schema1.HvRuntime{ImagePath: filepath.Join(uvmImagePath, `UtilityVM`)}
			}
		} else {
			// Hosting system was supplied, so is v2 Xenon.
			v2Container.Storage.Path = coi.Spec.Root.Path
			if coi.HostingSystem.OS() == "windows" {
				layers, err := computeV2Layers(coi.HostingSystem, coi.Spec.Windows.LayerFolders[:len(coi.Spec.Windows.LayerFolders)-1])
				if err != nil {
					return nil, err
				}
				v2Container.Storage.Layers = layers
			}
		}
	}

	if coi.HostingSystem == nil { // Argon v1 or v2
		for _, layerPath := range coi.Spec.Windows.LayerFolders[:len(coi.Spec.Windows.LayerFolders)-1] {
			layerID, err := wclayer.LayerID(layerPath)
			if err != nil {
				return nil, err
			}
			v1.Layers = append(v1.Layers, schema1.Layer{ID: layerID.String(), Path: layerPath})
			v2Container.Storage.Layers = append(v2Container.Storage.Layers, hcsschema.Layer{Id: layerID.String(), Path: layerPath})
		}
	}

	// Add the mounts as mapped directories or mapped pipes
	// TODO: Mapped pipes to add in v2 schema.
	var (
		mdsv1 []schema1.MappedDir
		mpsv1 []schema1.MappedPipe
		mdsv2 []hcsschema.MappedDirectory
		mpsv2 []hcsschema.MappedPipe
	)
	for _, mount := range coi.Spec.Mounts {
		const pipePrefix = `\\.\pipe\`
		if mount.Type != "" {
			return nil, fmt.Errorf("invalid container spec - Mount.Type '%s' must not be set", mount.Type)
		}
		if strings.HasPrefix(strings.ToLower(mount.Destination), pipePrefix) {
			mpsv1 = append(mpsv1, schema1.MappedPipe{HostPath: mount.Source, ContainerPipeName: mount.Destination[len(pipePrefix):]})
			mpsv2 = append(mpsv2, hcsschema.MappedPipe{HostPath: mount.Source, ContainerPipeName: mount.Destination[len(pipePrefix):]})
		} else {
			readOnly := false
			for _, o := range mount.Options {
				if strings.ToLower(o) == "ro" {
					readOnly = true
				}
			}
			mdv1 := schema1.MappedDir{HostPath: mount.Source, ContainerPath: mount.Destination, ReadOnly: readOnly}
			mdv2 := hcsschema.MappedDirectory{ContainerPath: mount.Destination, ReadOnly: readOnly}
			if coi.HostingSystem == nil {
				mdv2.HostPath = mount.Source
			} else {
				uvmPath, err := coi.HostingSystem.GetVSMBUvmPath(mount.Source)
				if err != nil {
					if err == uvm.ErrNotAttached {
						// It could also be a scsi mount.
						uvmPath, err = coi.HostingSystem.GetScsiUvmPath(mount.Source)
						if err != nil {
							return nil, err
						}
					} else {
						return nil, err
					}
				}
				mdv2.HostPath = uvmPath
			}
			mdsv1 = append(mdsv1, mdv1)
			mdsv2 = append(mdsv2, mdv2)
		}
	}

	v1.MappedDirectories = mdsv1
	v2Container.MappedDirectories = mdsv2
	if len(mpsv1) > 0 && osversion.Get().Build < osversion.RS3 {
		return nil, fmt.Errorf("named pipe mounts are not supported on this version of Windows")
	}
	v1.MappedPipes = mpsv1
	v2Container.MappedPipes = mpsv2

	// Put the v2Container object as a HostedSystem for a Xenon, or directly in the schema for an Argon.
	if coi.HostingSystem == nil {
		v2.Container = v2Container
	} else {
		v2.HostingSystemId = coi.HostingSystem.ID()
		v2.HostedSystem = &hcsschema.HostedSystem{
			SchemaVersion: schemaversion.SchemaV21(),
			Container:     v2Container,
		}
	}

	if schemaversion.IsV10(coi.actualSchemaVersion) {
		return v1, nil
	}

	return v2, nil
}
