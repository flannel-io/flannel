package lcow

import (
	"fmt"
	"io"
	//	"os"

	"github.com/Microsoft/hcsshim/internal/uvm"
	//	specs "github.com/opencontainers/runtime-spec/specs-go"
	//	"github.com/sirupsen/logrus"
)

// VhdToTar does what is says - it exports a VHD in a specified
// folder (either a read-only layer.vhd, or a read-write scratch vhdx) to a
// ReadCloser containing a tar-stream of the layers contents.
func VhdToTar(lcowUVM *uvm.UtilityVM, vhdFile string, uvmMountPath string, isContainerScratch bool, vhdSize int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented yet")
	//	logrus.Debugf("hcsshim: VhdToTar: %s isScratch: %t", vhdFile, isContainerScratch)

	//	if lcowUVM == nil {
	//		return nil, fmt.Errorf("cannot VhdToTar as no utility VM is in configuration")
	//	}

	//	//defer uvm.DebugLCOWGCS()

	//	vhdHandle, err := os.Open(vhdFile)
	//	if err != nil {
	//		return nil, fmt.Errorf("hcsshim: VhdToTar: failed to open %s: %s", vhdFile, err)
	//	}
	//	defer vhdHandle.Close()
	//	logrus.Debugf("hcsshim: VhdToTar: exporting %s, size %d, isScratch %t", vhdHandle.Name(), vhdSize, isContainerScratch)

	//	// Different binary depending on whether a RO layer or a RW scratch
	//	command := "vhd2tar"
	//	if isContainerScratch {
	//		command = fmt.Sprintf("exportSandbox -path %s", uvmMountPath)
	//	}

	//	//	tar2vhd, byteCounts, err := lcowUVM.CreateProcess(&uvm.ProcessOptions{
	//	//		Process: &specs.Process{Args: []string{"tar2vhd"}},
	//	//		Stdin:   reader,
	//	//		Stdout:  outFile,
	//	//	})

	//	// Start the binary in the utility VM
	//	proc, stdin, stdout, _, err := config.createLCOWUVMProcess(command)
	//	if err != nil {
	//		return nil, fmt.Errorf("hcsshim: VhdToTar: %s: failed to create utils process %s: %s", vhdHandle.Name(), command, err)
	//	}

	//	if !isContainerScratch {
	//		// Send the VHD contents to the utility VM processes stdin handle if not a container scratch
	//		logrus.Debugf("hcsshim: VhdToTar: copying the layer VHD into the utility VM")
	//		if _, err = copyWithTimeout(stdin, vhdHandle, vhdSize, processOperationTimeoutSeconds, fmt.Sprintf("vhdtotarstream: sending %s to %s", vhdHandle.Name(), command)); err != nil {
	//			proc.Close()
	//			return nil, fmt.Errorf("hcsshim: VhdToTar: %s: failed to copyWithTimeout on the stdin pipe (to utility VM): %s", vhdHandle.Name(), err)
	//		}
	//	}

	//	// Start a goroutine which copies the stdout (ie the tar stream)
	//	reader, writer := io.Pipe()
	//	go func() {
	//		defer writer.Close()
	//		defer proc.Close()
	//		logrus.Debugf("hcsshim: VhdToTar: copying tar stream back from the utility VM")
	//		bytes, err := copyWithTimeout(writer, stdout, vhdSize, processOperationTimeoutSeconds, fmt.Sprintf("vhdtotarstream: copy tarstream from %s", command))
	//		if err != nil {
	//			logrus.Errorf("hcsshim: VhdToTar: %s:  copyWithTimeout on the stdout pipe (from utility VM) failed: %s", vhdHandle.Name(), err)
	//		}
	//		logrus.Debugf("hcsshim: VhdToTar: copied %d bytes of the tarstream of %s from the utility VM", bytes, vhdHandle.Name())
	//	}()

	//	// Return the read-side of the pipe connected to the goroutine which is reading from the stdout of the process in the utility VM
	//	return reader, nil
}
