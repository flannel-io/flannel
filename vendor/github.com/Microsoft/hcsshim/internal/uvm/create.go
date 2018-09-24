package uvm

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/mergemaps"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/internal/schemaversion"
	"github.com/Microsoft/hcsshim/internal/uvmfolder"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/Microsoft/hcsshim/internal/wcow"
	"github.com/linuxkit/virtsock/pkg/hvsock"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

type PreferredRootFSType int

const (
	PreferredRootFSTypeInitRd = 0
	PreferredRootFSTypeVHD    = 1

	initrdFile = "initrd.img"
	vhdFile    = "rootfs.vhd"
)

// UVMOptions are the set of options passed to Create() to create a utility vm.
type UVMOptions struct {
	ID                      string                  // Identifier for the uvm. Defaults to generated GUID.
	Owner                   string                  // Specifies the owner. Defaults to executable name.
	OperatingSystem         string                  // "windows" or "linux".
	Resources               *specs.WindowsResources // Optional resources for the utility VM. Supports Memory.limit and CPU.Count only currently. // TODO consider extending?
	AdditionHCSDocumentJSON string                  // Optional additional JSON to merge into the HCS document prior

	// WCOW specific parameters
	LayerFolders []string // Set of folders for base layers and scratch. Ordered from top most read-only through base read-only layer, followed by scratch

	// LCOW specific parameters
	BootFilesPath         string               // Folder in which kernel and root file system reside. Defaults to \Program Files\Linux Containers
	KernelFile            string               // Filename under BootFilesPath for the kernel. Defaults to `kernel`
	RootFSFile            string               // Filename under BootFilesPath for the UVMs root file system. Defaults are `initrd.img` or `rootfs.vhd`.
	PreferredRootFSType   *PreferredRootFSType // Controls searching for the RootFSFile.
	KernelBootOptions     string               // Additional boot options for the kernel
	EnableGraphicsConsole bool                 // If true, enable a graphics console for the utility VM
	ConsolePipe           string               // The named pipe path to use for the serial console.  eg \\.\pipe\vmpipe
	VPMemDeviceCount      *int32               // Number of VPMem devices. Limit at 128. If booting UVM from VHD, device 0 is taken. LCOW Only.
	SCSIControllerCount   *int                 // The number of SCSI controllers. Defaults to 1 if omitted. Currently we only support 0 or 1.
}

const linuxLogVsockPort = 109

// Create creates an HCS compute system representing a utility VM.
//
// WCOW Notes:
//   - If the scratch folder does not exist, it will be created
//   - If the scratch folder does not contain `sandbox.vhdx` it will be created based on the system template located in the layer folders.
//   - The scratch is always attached to SCSI 0:0
//
func Create(opts *UVMOptions) (_ *UtilityVM, err error) {
	logrus.Debugf("uvm::Create %+v", opts)

	if opts == nil {
		return nil, fmt.Errorf("no options supplied to create")
	}

	uvm := &UtilityVM{
		id:              opts.ID,
		owner:           opts.Owner,
		operatingSystem: opts.OperatingSystem,
	}

	uvmFolder := "" // Windows

	if opts.OperatingSystem != "linux" && opts.OperatingSystem != "windows" {
		logrus.Debugf("uvm::Create Unsupported OS")
		return nil, fmt.Errorf("unsupported operating system %q", opts.OperatingSystem)
	}

	// Defaults if omitted by caller.
	// TODO: Change this. Don't auto generate ID if omitted. Avoids the chicken-and-egg problem
	if uvm.id == "" {
		uvm.id = guid.New().String()
	}
	if uvm.owner == "" {
		uvm.owner = filepath.Base(os.Args[0])
	}

	attachments := make(map[string]hcsschema.Attachment)
	scsi := make(map[string]hcsschema.Scsi)
	uvm.scsiControllerCount = 1
	var actualRootFSType PreferredRootFSType = PreferredRootFSTypeInitRd // TODO Should we switch to VPMem/VHD as default?

	if uvm.operatingSystem == "windows" {
		if len(opts.LayerFolders) < 2 {
			return nil, fmt.Errorf("at least 2 LayerFolders must be supplied")
		}
		if opts.VPMemDeviceCount != nil {
			return nil, fmt.Errorf("cannot specify VPMemDeviceCount for Windows utility VMs")
		}

		var err error
		uvmFolder, err = uvmfolder.LocateUVMFolder(opts.LayerFolders)
		if err != nil {
			return nil, fmt.Errorf("failed to locate utility VM folder from layer folders: %s", err)
		}

		// TODO: BUGBUG Remove this. @jhowardmsft
		//       It should be the responsiblity of the caller to do the creation and population.
		//       - Update runhcs too (vm.go).
		//       - Remove comment in function header
		//       - Update tests that rely on this current behaviour.
		// Create the RW scratch in the top-most layer folder, creating the folder if it doesn't already exist.
		scratchFolder := opts.LayerFolders[len(opts.LayerFolders)-1]
		logrus.Debugf("uvm::createWCOW scratch folder: %s", scratchFolder)

		// Create the directory if it doesn't exist
		if _, err := os.Stat(scratchFolder); os.IsNotExist(err) {
			logrus.Debugf("uvm::createWCOW Creating folder: %s ", scratchFolder)
			if err := os.MkdirAll(scratchFolder, 0777); err != nil {
				return nil, fmt.Errorf("failed to create utility VM scratch folder: %s", err)
			}
		}

		// Create sandbox.vhdx in the scratch folder based on the template, granting the correct permissions to it
		if _, err := os.Stat(filepath.Join(scratchFolder, `sandbox.vhdx`)); os.IsNotExist(err) {
			if err := wcow.CreateUVMScratch(uvmFolder, scratchFolder, uvm.id); err != nil {
				return nil, fmt.Errorf("failed to create scratch: %s", err)
			}
		}

		// We attach the scratch to SCSI 0:0
		attachments["0"] = hcsschema.Attachment{
			Path:  filepath.Join(scratchFolder, "sandbox.vhdx"),
			Type_: "VirtualDisk",
		}
		scsi["0"] = hcsschema.Scsi{Attachments: attachments}
		uvm.scsiLocations[0][0].hostPath = attachments["0"].Path
	} else {
		uvm.vpmemMax = DefaultVPMEM
		if opts.VPMemDeviceCount != nil {
			if *opts.VPMemDeviceCount > MaxVPMEM || *opts.VPMemDeviceCount < 0 {
				return nil, fmt.Errorf("vpmem device count must between 0 and %d", MaxVPMEM)
			}
			uvm.vpmemMax = *opts.VPMemDeviceCount
			logrus.Debugln("uvm::Create:: uvm.vpmemMax=", uvm.vpmemMax)
		}

		scsi["0"] = hcsschema.Scsi{Attachments: attachments}
		uvm.scsiControllerCount = 1
		if opts.SCSIControllerCount != nil {
			if *opts.SCSIControllerCount < 0 || *opts.SCSIControllerCount > 1 {
				return nil, fmt.Errorf("SCSI controller count must be 0 or 1") // Future extension here for up to 4
			}
			uvm.scsiControllerCount = *opts.SCSIControllerCount
			if uvm.scsiControllerCount == 0 {
				scsi = nil
			}
			logrus.Debugln("uvm::Create:: uvm.scsiControllerCount=", uvm.scsiControllerCount)
		}
		if opts.BootFilesPath == "" {
			opts.BootFilesPath = filepath.Join(os.Getenv("ProgramFiles"), "Linux Containers")
		}
		if opts.KernelFile == "" {
			opts.KernelFile = "kernel"
		}
		if _, err := os.Stat(filepath.Join(opts.BootFilesPath, opts.KernelFile)); os.IsNotExist(err) {
			return nil, fmt.Errorf("kernel '%s' not found", filepath.Join(opts.BootFilesPath, opts.KernelFile))
		}

		if opts.RootFSFile == "" {
			if opts.PreferredRootFSType != nil {
				actualRootFSType = *opts.PreferredRootFSType
				if actualRootFSType != PreferredRootFSTypeInitRd && actualRootFSType != PreferredRootFSTypeVHD {
					return nil, fmt.Errorf("invalid PreferredRootFSType")
				}
			}

			switch actualRootFSType {
			case PreferredRootFSTypeInitRd:
				if _, err := os.Stat(filepath.Join(opts.BootFilesPath, initrdFile)); os.IsNotExist(err) {
					return nil, fmt.Errorf("initrd not found")
				}
				opts.RootFSFile = initrdFile
			case PreferredRootFSTypeVHD:
				if _, err := os.Stat(filepath.Join(opts.BootFilesPath, vhdFile)); os.IsNotExist(err) {
					return nil, fmt.Errorf("rootfs.vhd not found")
				}
				opts.RootFSFile = vhdFile
			}
		} else {
			// Determine the root FS type by the extension of the explicitly supplied RootFSFile
			if _, err := os.Stat(filepath.Join(opts.BootFilesPath, opts.RootFSFile)); os.IsNotExist(err) {
				return nil, fmt.Errorf("%s not found under %s", opts.RootFSFile, opts.BootFilesPath)
			}
			switch strings.ToLower(filepath.Ext(opts.RootFSFile)) {
			case "vhd", "vhdx":
				actualRootFSType = PreferredRootFSTypeVHD
			case "img":
				actualRootFSType = PreferredRootFSTypeInitRd
			default:
				return nil, fmt.Errorf("unsupported filename extension for RootFSFile")
			}
		}
	}

	memory := int32(1024)
	processors := int32(2)
	if runtime.NumCPU() == 1 {
		processors = 1
	}
	if opts.Resources != nil {
		if opts.Resources.Memory != nil && opts.Resources.Memory.Limit != nil {
			memory = int32(*opts.Resources.Memory.Limit / 1024 / 1024) // OCI spec is in bytes. HCS takes MB
		}
		if opts.Resources.CPU != nil && opts.Resources.CPU.Count != nil {
			processors = int32(*opts.Resources.CPU.Count)
		}
	}

	vm := &hcsschema.VirtualMachine{
		Chipset: &hcsschema.Chipset{
			Uefi: &hcsschema.Uefi{},
		},

		ComputeTopology: &hcsschema.Topology{
			Memory: &hcsschema.Memory2{
				AllowOvercommit: true,
				SizeInMB:        memory,
			},
			Processor: &hcsschema.Processor2{
				Count: processors,
			},
		},

		GuestConnection: &hcsschema.GuestConnection{},

		Devices: &hcsschema.Devices{
			Scsi: scsi,
			HvSocket: &hcsschema.HvSocket2{
				HvSocketConfig: &hcsschema.HvSocketSystemConfig{
					// Allow administrators and SYSTEM to bind to vsock sockets
					// so that we can create a GCS log socket.
					DefaultBindSecurityDescriptor: "D:P(A;;FA;;;SY)(A;;FA;;;BA)",
				},
			},
		},
	}

	hcsDocument := &hcsschema.ComputeSystem{
		Owner:          uvm.owner,
		SchemaVersion:  schemaversion.SchemaV21(),
		VirtualMachine: vm,
	}

	if uvm.operatingSystem == "windows" {
		vm.Chipset.Uefi.BootThis = &hcsschema.UefiBootEntry{
			DevicePath: `\EFI\Microsoft\Boot\bootmgfw.efi`,
			DeviceType: "VmbFs",
		}
		vm.Devices.VirtualSmb = &hcsschema.VirtualSmb{
			DirectFileMappingInMB: 1024, // Sensible default, but could be a tuning parameter somewhere
			Shares: []hcsschema.VirtualSmbShare{
				{
					Name: "os",
					Path: filepath.Join(uvmFolder, `UtilityVM\Files`),
					Options: &hcsschema.VirtualSmbShareOptions{
						ReadOnly:            true,
						PseudoOplocks:       true,
						TakeBackupPrivilege: true,
						CacheIo:             true,
						ShareRead:           true,
					},
				},
			},
		}
	} else {
		vmDebugging := false
		vm.GuestConnection.UseVsock = true
		vm.GuestConnection.UseConnectedSuspend = true
		vm.Devices.VirtualSmb = &hcsschema.VirtualSmb{
			Shares: []hcsschema.VirtualSmbShare{
				{
					Name: "os",
					Path: opts.BootFilesPath,
					Options: &hcsschema.VirtualSmbShareOptions{
						ReadOnly:            true,
						TakeBackupPrivilege: true,
						CacheIo:             true,
						ShareRead:           true,
					},
				},
			},
		}

		if uvm.vpmemMax > 0 {
			vm.Devices.VirtualPMem = &hcsschema.VirtualPMemController{
				MaximumCount: uvm.vpmemMax,
			}
		}

		kernelArgs := "initrd=/" + opts.RootFSFile
		if actualRootFSType == PreferredRootFSTypeVHD {
			kernelArgs = "root=/dev/pmem0 init=/init"
		}

		// Support for VPMem VHD(X) booting rather than initrd..
		if actualRootFSType == PreferredRootFSTypeVHD {
			if uvm.vpmemMax == 0 {
				return nil, fmt.Errorf("PreferredRootFSTypeVHD requess at least one VPMem device")
			}
			imageFormat := "Vhd1"
			if strings.ToLower(filepath.Ext(opts.RootFSFile)) == "vhdx" {
				imageFormat = "Vhdx"
			}
			vm.Devices.VirtualPMem.Devices = map[string]hcsschema.VirtualPMemDevice{
				"0": {
					HostPath:    filepath.Join(opts.BootFilesPath, opts.RootFSFile),
					ReadOnly:    true,
					ImageFormat: imageFormat,
				},
			}
			if err := wclayer.GrantVmAccess(uvm.id, filepath.Join(opts.BootFilesPath, opts.RootFSFile)); err != nil {
				return nil, fmt.Errorf("faied to grantvmaccess to %s: %s", filepath.Join(opts.BootFilesPath, opts.RootFSFile), err)
			}
			// Add to our internal structure
			uvm.vpmemDevices[0] = vpmemInfo{
				hostPath: opts.RootFSFile,
				uvmPath:  "/",
				refCount: 1,
			}
		}

		if opts.ConsolePipe != "" {
			vmDebugging = true
			kernelArgs += " console=ttyS0,115200"
			vm.Devices.ComPorts = map[string]hcsschema.ComPort{
				"0": { // Which is actually COM1
					NamedPipe: opts.ConsolePipe,
				},
			}
		}

		if opts.EnableGraphicsConsole {
			vmDebugging = true
			kernelArgs += " console=tty"
			vm.Devices.Keyboard = &hcsschema.Keyboard{}
			vm.Devices.EnhancedModeVideo = &hcsschema.EnhancedModeVideo{}
			vm.Devices.VideoMonitor = &hcsschema.VideoMonitor{}
		}

		if !vmDebugging {
			// Terminate the VM if there is a kernel panic.
			kernelArgs += " panic=-1"
		}

		if opts.KernelBootOptions != "" {
			kernelArgs += " " + opts.KernelBootOptions
		}

		// Start GCS with stderr pointing to the vsock port created below in
		// order to forward guest logs to logrus.
		initArgs := fmt.Sprintf("/bin/vsockexec -e %d /bin/gcs -log-format json -loglevel %s",
			linuxLogVsockPort,
			logrus.StandardLogger().Level.String())

		if vmDebugging {
			// Launch a shell on the console.
			initArgs = `sh -c "` + initArgs + ` & exec sh"`
		}

		kernelArgs += ` -- ` + initArgs
		vm.Chipset.Uefi.BootThis = &hcsschema.UefiBootEntry{
			DevicePath:   `\` + opts.KernelFile,
			DeviceType:   "VmbFs",
			OptionalData: kernelArgs,
		}
	}

	fullDoc, err := mergemaps.MergeJSON(hcsDocument, ([]byte)(opts.AdditionHCSDocumentJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to merge additional JSON '%s': %s", opts.AdditionHCSDocumentJSON, err)
	}

	hcsSystem, err := hcs.CreateComputeSystem(uvm.id, fullDoc)
	if err != nil {
		logrus.Debugln("failed to create UVM: ", err)
		return nil, err
	}

	uvm.hcsSystem = hcsSystem
	defer func() {
		if err != nil {
			uvm.Close()
		}
	}()

	if uvm.operatingSystem == "linux" {
		// Create a socket that the GCS can send logrus log data to.
		uvm.gcslog, err = uvm.listenVsock(linuxLogVsockPort)
		if err != nil {
			return nil, err
		}
	}

	return uvm, nil
}

func (uvm *UtilityVM) listenVsock(port uint32) (net.Listener, error) {
	properties, err := uvm.hcsSystem.Properties()
	if err != nil {
		return nil, err
	}
	vmID, err := hvsock.GUIDFromString(properties.RuntimeID)
	if err != nil {
		return nil, err
	}
	serviceID, _ := hvsock.GUIDFromString("00000000-facb-11e6-bd58-64006a7986d3")
	binary.LittleEndian.PutUint32(serviceID[0:4], port)
	return hvsock.Listen(hvsock.Addr{VMID: vmID, ServiceID: serviceID})
}

// ID returns the ID of the VM's compute system.
func (uvm *UtilityVM) ID() string {
	return uvm.hcsSystem.ID()
}

// OS returns the operating system of the utility VM.
func (uvm *UtilityVM) OS() string {
	return uvm.operatingSystem
}

// Close terminates and releases resources associated with the utility VM.
func (uvm *UtilityVM) Close() error {
	uvm.Terminate()
	if uvm.gcslog != nil {
		uvm.gcslog.Close()
		uvm.gcslog = nil
	}
	err := uvm.hcsSystem.Close()
	uvm.hcsSystem = nil
	return err
}
