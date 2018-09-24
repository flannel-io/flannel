package lcow

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Microsoft/hcsshim/internal/uvm"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// TarToVhd streams a tarstream contained in an io.Reader to a fixed vhd file
func TarToVhd(lcowUVM *uvm.UtilityVM, targetVHDFile string, reader io.Reader) (int64, error) {
	logrus.Debugf("hcsshim: TarToVhd: %s", targetVHDFile)

	if lcowUVM == nil {
		return 0, fmt.Errorf("no utility VM passed")
	}

	//defer uvm.DebugLCOWGCS()

	outFile, err := os.Create(targetVHDFile)
	if err != nil {
		return 0, fmt.Errorf("tar2vhd failed to create %s: %s", targetVHDFile, err)
	}
	defer outFile.Close()
	// BUGBUG Delete the file on failure

	tar2vhd, byteCounts, err := CreateProcess(&ProcessOptions{
		HCSSystem:         lcowUVM.ComputeSystem(),
		Process:           &specs.Process{Args: []string{"tar2vhd"}},
		CreateInUtilityVm: true,
		Stdin:             reader,
		Stdout:            outFile,
		CopyTimeout:       2 * time.Minute,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to start tar2vhd for %s: %s", targetVHDFile, err)
	}
	defer tar2vhd.Close()

	logrus.Debugf("hcsshim: TarToVhd: %s created, %d bytes", targetVHDFile, byteCounts.Out)
	return byteCounts.Out, err
}
