package uvm

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/mergemaps"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/internal/schemaversion"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/linuxkit/virtsock/pkg/hvsock"
	"github.com/sirupsen/logrus"
)

type PreferredRootFSType int

const (
	PreferredRootFSTypeInitRd PreferredRootFSType = iota
	PreferredRootFSTypeVHD
)

// OutputHandler is used to process the output from the program run in the UVM.
type OutputHandler func(io.Reader)

const (
	// InitrdFile is the default file name for an initrd.img used to boot LCOW.
	InitrdFile = "initrd.img"
	// VhdFile is the default file name for a rootfs.vhd used to boot LCOW.
	VhdFile = "rootfs.vhd"
	// KernelFile is the default file name for a kernel used to boot LCOW.
	KernelFile = "kernel"
	// UncompressedKernelFile is the default file name for an uncompressed
	// kernel used to boot LCOW with KernelDirect.
	UncompressedKernelFile = "vmlinux"
)

// OptionsLCOW are the set of options passed to CreateLCOW() to create a utility vm.
type OptionsLCOW struct {
	*Options

	BootFilesPath         string              // Folder in which kernel and root file system reside. Defaults to \Program Files\Linux Containers
	KernelFile            string              // Filename under `BootFilesPath` for the kernel. Defaults to `kernel`
	KernelDirect          bool                // Skip UEFI and boot directly to `kernel`
	RootFSFile            string              // Filename under `BootFilesPath` for the UVMs root file system. Defaults to `InitrdFile`
	KernelBootOptions     string              // Additional boot options for the kernel
	EnableGraphicsConsole bool                // If true, enable a graphics console for the utility VM
	ConsolePipe           string              // The named pipe path to use for the serial console.  eg \\.\pipe\vmpipe
	SCSIControllerCount   uint32              // The number of SCSI controllers. Defaults to 1. Currently we only support 0 or 1.
	UseGuestConnection    bool                // Whether the HCS should connect to the UVM's GCS. Defaults to true
	ExecCommandLine       string              // The command line to exec from init. Defaults to GCS
	ForwardStdout         bool                // Whether stdout will be forwarded from the executed program. Defaults to false
	ForwardStderr         bool                // Whether stderr will be forwarded from the executed program. Defaults to true
	OutputHandler         OutputHandler       `json:"-"` // Controls how output received over HVSocket from the UVM is handled. Defaults to parsing output as logrus messages
	VPMemDeviceCount      uint32              // Number of VPMem devices. Defaults to `DefaultVPMEMCount`. Limit at 128. If booting UVM from VHD, device 0 is taken.
	VPMemSizeBytes        uint64              // Size of the VPMem devices. Defaults to `DefaultVPMemSizeBytes`.
	PreferredRootFSType   PreferredRootFSType // If `KernelFile` is `InitrdFile` use `PreferredRootFSTypeInitRd`. If `KernelFile` is `VhdFile` use `PreferredRootFSTypeVHD`
}

// NewDefaultOptionsLCOW creates the default options for a bootable version of
// LCOW.
//
// `id` the ID of the compute system. If not passed will generate a new GUID.
//
// `owner` the owner of the compute system. If not passed will use the
// executable files name.
func NewDefaultOptionsLCOW(id, owner string) *OptionsLCOW {
	// Use KernelDirect boot by default on all builds that support it.
	kernelDirectSupported := osversion.Get().Build >= 18286
	opts := &OptionsLCOW{
		Options: &Options{
			ID:                   id,
			Owner:                owner,
			MemorySizeInMB:       1024,
			AllowOvercommit:      true,
			EnableDeferredCommit: false,
			ProcessorCount:       defaultProcessorCount(),
		},
		BootFilesPath:         filepath.Join(os.Getenv("ProgramFiles"), "Linux Containers"),
		KernelFile:            KernelFile,
		KernelDirect:          kernelDirectSupported,
		RootFSFile:            InitrdFile,
		KernelBootOptions:     "",
		EnableGraphicsConsole: false,
		ConsolePipe:           "",
		SCSIControllerCount:   1,
		UseGuestConnection:    true,
		ExecCommandLine:       fmt.Sprintf("/bin/gcs -log-format json -loglevel %s", logrus.StandardLogger().Level.String()),
		ForwardStdout:         false,
		ForwardStderr:         true,
		OutputHandler:         parseLogrus,
		VPMemDeviceCount:      DefaultVPMEMCount,
		VPMemSizeBytes:        DefaultVPMemSizeBytes,
		PreferredRootFSType:   PreferredRootFSTypeInitRd,
	}

	if opts.ID == "" {
		opts.ID = guid.New().String()
	}
	if opts.Owner == "" {
		opts.Owner = filepath.Base(os.Args[0])
	}

	if _, err := os.Stat(filepath.Join(opts.BootFilesPath, VhdFile)); err == nil {
		// We have a rootfs.vhd in the boot files path. Use it over an initrd.img
		opts.RootFSFile = VhdFile
		opts.PreferredRootFSType = PreferredRootFSTypeVHD
	}

	if kernelDirectSupported {
		// KernelDirect supports uncompressed kernel if the kernel is present.
		// Default to uncompressed if on box. NOTE: If `kernel` is already
		// uncompressed and simply named 'kernel' it will still be used
		// uncompressed automatically.
		if _, err := os.Stat(filepath.Join(opts.BootFilesPath, UncompressedKernelFile)); err == nil {
			opts.KernelFile = UncompressedKernelFile
		}
	}
	return opts
}

const linuxLogVsockPort = 109

// CreateLCOW creates an HCS compute system representing a utility VM.
func CreateLCOW(opts *OptionsLCOW) (_ *UtilityVM, err error) {
	logrus.Debugf("uvm::CreateLCOW %+v", opts)

	// We dont serialize OutputHandler so if it is missing we need to put it back to the default.
	if opts.OutputHandler == nil {
		opts.OutputHandler = parseLogrus
	}

	uvm := &UtilityVM{
		id:                  opts.ID,
		owner:               opts.Owner,
		operatingSystem:     "linux",
		scsiControllerCount: opts.SCSIControllerCount,
		vpmemMaxCount:       opts.VPMemDeviceCount,
		vpmemMaxSizeBytes:   opts.VPMemSizeBytes,
	}

	kernelFullPath := filepath.Join(opts.BootFilesPath, opts.KernelFile)
	if _, err := os.Stat(kernelFullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("kernel: '%s' not found", kernelFullPath)
	}
	rootfsFullPath := filepath.Join(opts.BootFilesPath, opts.RootFSFile)
	if _, err := os.Stat(rootfsFullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("boot file: '%s' not found", rootfsFullPath)
	}

	if opts.SCSIControllerCount > 1 {
		return nil, fmt.Errorf("SCSI controller count must be 0 or 1") // Future extension here for up to 4
	}
	if opts.VPMemDeviceCount > MaxVPMEMCount {
		return nil, fmt.Errorf("vpmem device count cannot be greater than %d", MaxVPMEMCount)
	}
	if uvm.vpmemMaxCount > 0 {
		if opts.VPMemSizeBytes%4096 != 0 {
			return nil, fmt.Errorf("opts.VPMemSizeBytes must be a multiple of 4096")
		}
	} else {
		if opts.PreferredRootFSType == PreferredRootFSTypeVHD {
			return nil, fmt.Errorf("PreferredRootFSTypeVHD requires at least one VPMem device")
		}
	}
	if opts.KernelDirect && osversion.Get().Build < 18286 {
		return nil, fmt.Errorf("KernelDirectBoot is not support on builds older than 18286")
	}

	doc := &hcsschema.ComputeSystem{
		Owner:                             uvm.owner,
		SchemaVersion:                     schemaversion.SchemaV21(),
		ShouldTerminateOnLastHandleClosed: true,
		VirtualMachine: &hcsschema.VirtualMachine{
			StopOnReset: true,
			Chipset:     &hcsschema.Chipset{},
			ComputeTopology: &hcsschema.Topology{
				Memory: &hcsschema.Memory2{
					SizeInMB:             opts.MemorySizeInMB,
					AllowOvercommit:      opts.AllowOvercommit,
					EnableDeferredCommit: opts.EnableDeferredCommit,
				},
				Processor: &hcsschema.Processor2{
					Count: opts.ProcessorCount,
				},
			},
			Devices: &hcsschema.Devices{
				HvSocket: &hcsschema.HvSocket2{
					HvSocketConfig: &hcsschema.HvSocketSystemConfig{
						// Allow administrators and SYSTEM to bind to vsock sockets
						// so that we can create a GCS log socket.
						DefaultBindSecurityDescriptor: "D:P(A;;FA;;;SY)(A;;FA;;;BA)",
					},
				},
			},
		},
	}

	if opts.UseGuestConnection {
		doc.VirtualMachine.GuestConnection = &hcsschema.GuestConnection{
			UseVsock:            true,
			UseConnectedSuspend: true,
		}
	}

	if uvm.scsiControllerCount > 0 {
		// TODO: JTERRY75 - this should enumerate scsicount and add an entry per value.
		doc.VirtualMachine.Devices.Scsi = map[string]hcsschema.Scsi{
			"0": {
				Attachments: make(map[string]hcsschema.Attachment),
			},
		}
	}
	if uvm.vpmemMaxCount > 0 {
		doc.VirtualMachine.Devices.VirtualPMem = &hcsschema.VirtualPMemController{
			MaximumCount:     uvm.vpmemMaxCount,
			MaximumSizeBytes: uvm.vpmemMaxSizeBytes,
		}
	}

	var kernelArgs string
	switch opts.PreferredRootFSType {
	case PreferredRootFSTypeInitRd:
		if !opts.KernelDirect {
			kernelArgs = "initrd=/" + opts.RootFSFile
		}
	case PreferredRootFSTypeVHD:
		// Support for VPMem VHD(X) booting rather than initrd..
		kernelArgs = "root=/dev/pmem0 ro init=/init"
		imageFormat := "Vhd1"
		if strings.ToLower(filepath.Ext(opts.RootFSFile)) == "vhdx" {
			imageFormat = "Vhdx"
		}
		doc.VirtualMachine.Devices.VirtualPMem.Devices = map[string]hcsschema.VirtualPMemDevice{
			"0": {
				HostPath:    rootfsFullPath,
				ReadOnly:    true,
				ImageFormat: imageFormat,
			},
		}
		if err := wclayer.GrantVmAccess(uvm.id, rootfsFullPath); err != nil {
			return nil, fmt.Errorf("failed to grantvmaccess to %s: %s", rootfsFullPath, err)
		}
		// Add to our internal structure
		uvm.vpmemDevices[0] = vpmemInfo{
			hostPath: opts.RootFSFile,
			uvmPath:  "/",
			refCount: 1,
		}
	}

	vmDebugging := false
	if opts.ConsolePipe != "" {
		vmDebugging = true
		kernelArgs += " 8250_core.nr_uarts=1 8250_core.skip_txen_test=1 console=ttyS0,115200"
		doc.VirtualMachine.Devices.ComPorts = map[string]hcsschema.ComPort{
			"0": { // Which is actually COM1
				NamedPipe: opts.ConsolePipe,
			},
		}
	} else {
		kernelArgs += " 8250_core.nr_uarts=0"
	}

	if opts.EnableGraphicsConsole {
		vmDebugging = true
		kernelArgs += " console=tty"
		doc.VirtualMachine.Devices.Keyboard = &hcsschema.Keyboard{}
		doc.VirtualMachine.Devices.EnhancedModeVideo = &hcsschema.EnhancedModeVideo{}
		doc.VirtualMachine.Devices.VideoMonitor = &hcsschema.VideoMonitor{}
	}

	if !vmDebugging {
		// Terminate the VM if there is a kernel panic.
		kernelArgs += " panic=-1 quiet"
	}

	if opts.KernelBootOptions != "" {
		kernelArgs += " " + opts.KernelBootOptions
	}

	// With default options, run GCS with stderr pointing to the vsock port
	// created below in order to forward guest logs to logrus.
	initArgs := "/bin/vsockexec"

	if opts.ForwardStdout {
		initArgs += fmt.Sprintf(" -o %d", linuxLogVsockPort)
	}

	if opts.ForwardStderr {
		initArgs += fmt.Sprintf(" -e %d", linuxLogVsockPort)
	}

	initArgs += " " + opts.ExecCommandLine

	if vmDebugging {
		// Launch a shell on the console.
		initArgs = `sh -c "` + initArgs + ` & exec sh"`
	}

	kernelArgs += ` pci=off brd.rd_nr=0 pmtmr=0 -- ` + initArgs

	if !opts.KernelDirect {
		doc.VirtualMachine.Chipset.Uefi = &hcsschema.Uefi{
			BootThis: &hcsschema.UefiBootEntry{
				DevicePath:    `\` + opts.KernelFile,
				DeviceType:    "VmbFs",
				VmbFsRootPath: opts.BootFilesPath,
				OptionalData:  kernelArgs,
			},
		}
	} else {
		doc.VirtualMachine.Chipset.LinuxKernelDirect = &hcsschema.LinuxKernelDirect{
			KernelFilePath: kernelFullPath,
			KernelCmdLine:  kernelArgs,
		}
		if opts.PreferredRootFSType == PreferredRootFSTypeInitRd {
			doc.VirtualMachine.Chipset.LinuxKernelDirect.InitRdPath = rootfsFullPath
		}
	}

	fullDoc, err := mergemaps.MergeJSON(doc, ([]byte)(opts.AdditionHCSDocumentJSON))
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

	// Create a socket that the executed program can send to. This is usually
	// used by GCS to send log data.
	if opts.ForwardStdout || opts.ForwardStderr {
		uvm.outputHandler = opts.OutputHandler
		uvm.outputProcessingDone = make(chan struct{})
		uvm.outputListener, err = uvm.listenVsock(linuxLogVsockPort)
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

// PMemMaxSizeBytes returns the maximum size of a PMEM layer (LCOW)
func (uvm *UtilityVM) PMemMaxSizeBytes() uint64 {
	return uvm.vpmemMaxSizeBytes
}
