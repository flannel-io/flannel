// +build functional lcow

package functional

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcsoci"
	"github.com/Microsoft/hcsshim/internal/lcow"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/test/functional/utilities"
)

// TestLCOWUVMNoSCSINoVPMemInitrd starts an LCOW utility VM without a SCSI controller and
// no VPMem device. Uses initrd.
func TestLCOWUVMNoSCSINoVPMemInitrd(t *testing.T) {
	opts := uvm.NewDefaultOptionsLCOW(t.Name(), "")
	opts.SCSIControllerCount = 0
	opts.VPMemDeviceCount = 0
	opts.PreferredRootFSType = uvm.PreferredRootFSTypeInitRd
	opts.RootFSFile = uvm.InitrdFile

	testLCOWUVMNoSCSISingleVPMem(t, opts, fmt.Sprintf("Command line: initrd=/%s", opts.RootFSFile))
}

// TestLCOWUVMNoSCSISingleVPMemVHD starts an LCOW utility VM without a SCSI controller and
// only a single VPMem device. Uses VPMEM VHD
func TestLCOWUVMNoSCSISingleVPMemVHD(t *testing.T) {
	opts := uvm.NewDefaultOptionsLCOW(t.Name(), "")
	opts.SCSIControllerCount = 0
	opts.VPMemDeviceCount = 1
	opts.PreferredRootFSType = uvm.PreferredRootFSTypeVHD
	opts.RootFSFile = uvm.VhdFile

	testLCOWUVMNoSCSISingleVPMem(t, opts, `Command line: root=/dev/pmem0 init=/init`)
}

func testLCOWUVMNoSCSISingleVPMem(t *testing.T, opts *uvm.OptionsLCOW, expected string) {
	testutilities.RequiresBuild(t, osversion.RS5)
	lcowUVM := testutilities.CreateLCOWUVMFromOpts(t, opts)
	defer lcowUVM.Close()
	out, err := exec.Command(`hcsdiag`, `exec`, `-uvm`, lcowUVM.ID(), `dmesg`).Output() // TODO: Move the CreateProcess.
	if err != nil {
		t.Fatal(string(err.(*exec.ExitError).Stderr))
	}
	if !strings.Contains(string(out), expected) {
		t.Fatalf("Expected dmesg output to have %q: %s", expected, string(out))
	}
}

// TestLCOWTimeUVMStartVHD starts/terminates a utility VM booting from VPMem-
// attached root filesystem a number of times.
func TestLCOWTimeUVMStartVHD(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)

	testLCOWTimeUVMStart(t, false, uvm.PreferredRootFSTypeVHD)
}

// TestLCOWUVMStart_KernelDirect_VHD starts/terminates a utility VM booting from
// VPMem- attached root filesystem a number of times starting from the Linux
// Kernel directly and skipping EFI.
func TestLCOWUVMStart_KernelDirect_VHD(t *testing.T) {
	testutilities.RequiresBuild(t, 18286)

	testLCOWTimeUVMStart(t, true, uvm.PreferredRootFSTypeVHD)
}

// TestLCOWTimeUVMStartInitRD starts/terminates a utility VM booting from initrd-
// attached root file system a number of times.
func TestLCOWTimeUVMStartInitRD(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)

	testLCOWTimeUVMStart(t, false, uvm.PreferredRootFSTypeInitRd)
}

// TestLCOWUVMStart_KernelDirect_InitRd starts/terminates a utility VM booting
// from initrd- attached root file system a number of times starting from the
// Linux Kernel directly and skipping EFI.
func TestLCOWUVMStart_KernelDirect_InitRd(t *testing.T) {
	testutilities.RequiresBuild(t, 18286)

	testLCOWTimeUVMStart(t, true, uvm.PreferredRootFSTypeInitRd)
}

func testLCOWTimeUVMStart(t *testing.T, kernelDirect bool, rfsType uvm.PreferredRootFSType) {
	for i := 0; i < 3; i++ {
		opts := uvm.NewDefaultOptionsLCOW(t.Name(), "")
		opts.KernelDirect = kernelDirect
		opts.VPMemDeviceCount = 32
		opts.PreferredRootFSType = rfsType
		switch opts.PreferredRootFSType {
		case uvm.PreferredRootFSTypeInitRd:
			opts.RootFSFile = uvm.InitrdFile
		case uvm.PreferredRootFSTypeVHD:
			opts.RootFSFile = uvm.VhdFile
		}

		lcowUVM := testutilities.CreateLCOWUVMFromOpts(t, opts)
		lcowUVM.Close()
	}
}

func TestLCOWSimplePodScenario(t *testing.T) {
	t.Skip("Doesn't work quite yet")
	testutilities.RequiresBuild(t, osversion.RS5)
	alpineLayers := testutilities.LayerFolders(t, "alpine")

	cacheDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(cacheDir)
	cacheFile := filepath.Join(cacheDir, "cache.vhdx")

	// This is what gets mounted into /tmp/scratch
	uvmScratchDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(uvmScratchDir)
	uvmScratchFile := filepath.Join(uvmScratchDir, "uvmscratch.vhdx")

	// Scratch for the first container
	c1ScratchDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(c1ScratchDir)
	c1ScratchFile := filepath.Join(c1ScratchDir, "sandbox.vhdx")

	// Scratch for the second container
	c2ScratchDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(c2ScratchDir)
	c2ScratchFile := filepath.Join(c2ScratchDir, "sandbox.vhdx")

	lcowUVM := testutilities.CreateLCOWUVM(t, "uvm")
	defer lcowUVM.Close()

	// Populate the cache and generate the scratch file for /tmp/scratch
	if err := lcow.CreateScratch(lcowUVM, uvmScratchFile, lcow.DefaultScratchSizeGB, cacheFile, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lcowUVM.AddSCSI(uvmScratchFile, `/tmp/scratch`, false); err != nil {
		t.Fatal(err)
	}

	// Now create the first containers sandbox, populate a spec
	if err := lcow.CreateScratch(lcowUVM, c1ScratchFile, lcow.DefaultScratchSizeGB, cacheFile, ""); err != nil {
		t.Fatal(err)
	}
	c1Spec := testutilities.GetDefaultLinuxSpec(t)
	c1Folders := append(alpineLayers, c1ScratchDir)
	c1Spec.Windows.LayerFolders = c1Folders
	c1Spec.Process.Args = []string{"echo", "hello", "lcow", "container", "one"}
	c1Opts := &hcsoci.CreateOptions{
		Spec:          c1Spec,
		HostingSystem: lcowUVM,
	}

	// Now create the second containers sandbox, populate a spec
	if err := lcow.CreateScratch(lcowUVM, c2ScratchFile, lcow.DefaultScratchSizeGB, cacheFile, ""); err != nil {
		t.Fatal(err)
	}
	c2Spec := testutilities.GetDefaultLinuxSpec(t)
	c2Folders := append(alpineLayers, c2ScratchDir)
	c2Spec.Windows.LayerFolders = c2Folders
	c2Spec.Process.Args = []string{"echo", "hello", "lcow", "container", "two"}
	c2Opts := &hcsoci.CreateOptions{
		Spec:          c2Spec,
		HostingSystem: lcowUVM,
	}

	// Create the two containers
	c1hcsSystem, c1Resources, err := CreateContainerTestWrapper(c1Opts)
	if err != nil {
		t.Fatal(err)
	}
	c2hcsSystem, c2Resources, err := CreateContainerTestWrapper(c2Opts)
	if err != nil {
		t.Fatal(err)
	}

	// Start them. In the UVM, they'll be in the created state from runc's perspective after this.eg
	/// # runc list
	//ID                                     PID         STATUS      BUNDLE         CREATED                        OWNER
	//3a724c2b-f389-5c71-0555-ebc6f5379b30   138         running     /run/gcs/c/1   2018-06-04T21:23:39.1253911Z   root
	//7a8229a0-eb60-b515-55e7-d2dd63ffae75   158         created     /run/gcs/c/2   2018-06-04T21:23:39.4249048Z   root
	if err := c1hcsSystem.Start(); err != nil {
		t.Fatal(err)
	}
	defer hcsoci.ReleaseResources(c1Resources, lcowUVM, true)

	if err := c2hcsSystem.Start(); err != nil {
		t.Fatal(err)
	}
	defer hcsoci.ReleaseResources(c2Resources, lcowUVM, true)

	// Start the init process in each container and grab it's stdout comparing to expected
	runInitProcess(t, c1hcsSystem, "hello lcow container one")
	runInitProcess(t, c2hcsSystem, "hello lcow container two")

}

// Helper to run the init process in an LCOW container; verify it exits with exit
// code 0; verify stderr is empty; check output is as expected.
func runInitProcess(t *testing.T, s *hcs.System, expected string) {
	var outB, errB bytes.Buffer
	p, bc, err := lcow.CreateProcess(&lcow.ProcessOptions{
		HCSSystem:   s,
		Stdout:      &outB,
		Stderr:      &errB,
		CopyTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if bc.Err != 0 {
		t.Fatalf("got %d bytes on stderr: %s", bc.Err, errB.String())
	}
	if strings.TrimSpace(outB.String()) != expected {
		t.Fatalf("got %q (%d) expecting %q", outB.String(), bc.Out, expected)
	}
}
