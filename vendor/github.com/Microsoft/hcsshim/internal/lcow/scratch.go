package lcow

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Microsoft/go-winio/vhd"
	"github.com/Microsoft/hcsshim/internal/copyfile"
	"github.com/Microsoft/hcsshim/internal/timeout"
	"github.com/Microsoft/hcsshim/internal/uvm"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// CreateScratch uses a utility VM to create an empty scratch disk of a requested size.
// It has a caching capability. If the cacheFile exists, and the request is for a default
// size, a copy of that is made to the target. If the size is non-default, or the cache file
// does not exist, it uses a utility VM to create target. It is the responsibility of the
// caller to synchronise simultaneous attempts to create the cache file.
func CreateScratch(lcowUVM *uvm.UtilityVM, destFile string, sizeGB uint32, cacheFile string, vmID string) error {

	if lcowUVM == nil {
		return fmt.Errorf("no uvm")
	}

	if lcowUVM.OS() != "linux" {
		return fmt.Errorf("CreateLCOWScratch requires a linux utility VM to operate!")
	}

	// Smallest we can accept is the default scratch size as we can't size down, only expand.
	if sizeGB < DefaultScratchSizeGB {
		sizeGB = DefaultScratchSizeGB
	}

	logrus.Debugf("hcsshim::CreateLCOWScratch: Dest:%s size:%dGB cache:%s", destFile, sizeGB, cacheFile)

	// Retrieve from cache if the default size and already on disk
	if cacheFile != "" && sizeGB == DefaultScratchSizeGB {
		if _, err := os.Stat(cacheFile); err == nil {
			if err := copyfile.CopyFile(cacheFile, destFile, false); err != nil {
				return fmt.Errorf("failed to copy cached file '%s' to '%s': %s", cacheFile, destFile, err)
			}
			logrus.Debugf("hcsshim::CreateLCOWScratch: %s fulfilled from cache (%s)", destFile, cacheFile)
			return nil
		}
	}

	// Create the VHDX
	if err := vhd.CreateVhdx(destFile, sizeGB, defaultVhdxBlockSizeMB); err != nil {
		return fmt.Errorf("failed to create VHDx %s: %s", destFile, err)
	}

	controller, lun, err := lcowUVM.AddSCSI(destFile, "", false) // No destination as not formatted
	if err != nil {
		return err
	}

	logrus.Debugf("hcsshim::CreateLCOWScratch: %s at C=%d L=%d", destFile, controller, lun)

	// Validate /sys/bus/scsi/devices/C:0:0:L exists as a directory

	startTime := time.Now()
	for {
		testdCommand := []string{"test", "-d", fmt.Sprintf("/sys/bus/scsi/devices/%d:0:0:%d", controller, lun)}
		testdProc, _, err := CreateProcess(&ProcessOptions{
			HCSSystem:         lcowUVM.ComputeSystem(),
			CreateInUtilityVm: true,
			CopyTimeout:       timeout.ExternalCommandToStart,
			Process:           &specs.Process{Args: testdCommand},
		})
		if err != nil {
			lcowUVM.RemoveSCSI(destFile)
			return fmt.Errorf("failed to run %+v following hot-add %s to utility VM: %s", testdCommand, destFile, err)
		}
		defer testdProc.Close()

		testdProc.WaitTimeout(timeout.ExternalCommandToComplete)
		testdExitCode, err := testdProc.ExitCode()
		if err != nil {
			lcowUVM.RemoveSCSI(destFile)
			return fmt.Errorf("failed to get exit code from from %+v following hot-add %s to utility VM: %s", testdCommand, destFile, err)
		}
		if testdExitCode != 0 {
			currentTime := time.Now()
			elapsedTime := currentTime.Sub(startTime)
			if elapsedTime > timeout.TestDRetryLoop {
				lcowUVM.RemoveSCSI(destFile)
				return fmt.Errorf("`%+v` return non-zero exit code (%d) following hot-add %s to utility VM", testdCommand, testdExitCode, destFile)
			}
		} else {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}

	// Get the device from under the block subdirectory by doing a simple ls. This will come back as (eg) `sda`
	var lsOutput bytes.Buffer
	lsCommand := []string{"ls", fmt.Sprintf("/sys/bus/scsi/devices/%d:0:0:%d/block", controller, lun)}
	lsProc, _, err := CreateProcess(&ProcessOptions{
		HCSSystem:         lcowUVM.ComputeSystem(),
		CreateInUtilityVm: true,
		CopyTimeout:       timeout.ExternalCommandToStart,
		Process:           &specs.Process{Args: lsCommand},
		Stdout:            &lsOutput,
	})

	if err != nil {
		lcowUVM.RemoveSCSI(destFile)
		return fmt.Errorf("failed to `%+v` following hot-add %s to utility VM: %s", lsCommand, destFile, err)
	}
	defer lsProc.Close()
	lsProc.WaitTimeout(timeout.ExternalCommandToComplete)
	lsExitCode, err := lsProc.ExitCode()
	if err != nil {
		lcowUVM.RemoveSCSI(destFile)
		return fmt.Errorf("failed to get exit code from `%+v` following hot-add %s to utility VM: %s", lsCommand, destFile, err)
	}
	if lsExitCode != 0 {
		lcowUVM.RemoveSCSI(destFile)
		return fmt.Errorf("`%+v` return non-zero exit code (%d) following hot-add %s to utility VM", lsCommand, lsExitCode, destFile)
	}
	device := fmt.Sprintf(`/dev/%s`, strings.TrimSpace(lsOutput.String()))
	logrus.Debugf("hcsshim: CreateExt4Vhdx: %s: device at %s", destFile, device)

	// Format it ext4
	mkfsCommand := []string{"mkfs.ext4", "-q", "-E", "lazy_itable_init=1", "-O", `^has_journal,sparse_super2,uninit_bg,^resize_inode`, device}
	var mkfsStderr bytes.Buffer
	mkfsProc, _, err := CreateProcess(&ProcessOptions{
		HCSSystem:         lcowUVM.ComputeSystem(),
		CreateInUtilityVm: true,
		CopyTimeout:       timeout.ExternalCommandToStart,
		Process:           &specs.Process{Args: mkfsCommand},
		Stderr:            &mkfsStderr,
	})
	if err != nil {
		lcowUVM.RemoveSCSI(destFile)
		return fmt.Errorf("failed to `%+v` following hot-add %s to utility VM: %s", mkfsCommand, destFile, err)
	}
	defer mkfsProc.Close()
	mkfsProc.WaitTimeout(timeout.ExternalCommandToComplete)
	mkfsExitCode, err := mkfsProc.ExitCode()
	if err != nil {
		lcowUVM.RemoveSCSI(destFile)
		return fmt.Errorf("failed to get exit code from `%+v` following hot-add %s to utility VM: %s", mkfsCommand, destFile, err)
	}
	if mkfsExitCode != 0 {
		lcowUVM.RemoveSCSI(destFile)
		return fmt.Errorf("`%+v` return non-zero exit code (%d) following hot-add %s to utility VM: %s", mkfsCommand, mkfsExitCode, destFile, strings.TrimSpace(mkfsStderr.String()))
	}

	// Hot-Remove before we copy it
	if err := lcowUVM.RemoveSCSI(destFile); err != nil {
		return fmt.Errorf("failed to hot-remove: %s", err)
	}

	// Populate the cache.
	if cacheFile != "" && (sizeGB == DefaultScratchSizeGB) {
		if err := copyfile.CopyFile(destFile, cacheFile, true); err != nil {
			return fmt.Errorf("failed to seed cache '%s' from '%s': %s", destFile, cacheFile, err)
		}
	}

	logrus.Debugf("hcsshim::CreateLCOWScratch: %s created (non-cache)", destFile)
	return nil
}
