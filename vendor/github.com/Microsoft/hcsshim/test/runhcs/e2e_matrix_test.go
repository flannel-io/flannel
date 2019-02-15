// +build integration

package runhcs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/Microsoft/go-winio/vhd"
	"github.com/Microsoft/hcsshim/osversion"
	runhcs "github.com/Microsoft/hcsshim/pkg/go-runhcs"
	testutilities "github.com/Microsoft/hcsshim/test/functional/utilities"
	runc "github.com/containerd/go-runc"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// These support matrix of runhcs.exe for end to end activations is quite
// complex. These tests attempt to codify a simple start test on each support
// host/guest/isolation type so that we can have at least minimal confidence
// when changing code that activations across all platforms still work.
//
// Host OS | Container OS  | Isolation
//
// RS1     | RS1           | V1 - Argon, Xenon
//
// RS3     | RS1           | V1 - Xenon
//         | RS3           | V1 - Argon, Xenon
//
// RS4     | RS1, RS3      | V1 - Xenon
//         | RS4           | V1 - Argon, Xenon
//
// RS5     | RS1, RS3, RS4 | V2 - UVM + Argon
//         | RS5           | V2 - Argon, UVM + Argon, UVM + Argon (s) (POD's)
//         | LCOW          | V2 - UVM + Linux Container, UVM + Linux Container (s) (POD's)

var _ = (runc.IO)(&testIO{})

type testIO struct {
	g *errgroup.Group

	or, ow  *os.File
	outBuff *bytes.Buffer

	er, ew  *os.File
	errBuff *bytes.Buffer
}

func newTestIO(t *testing.T) *testIO {
	var err error
	tio := &testIO{
		outBuff: &bytes.Buffer{},
		errBuff: &bytes.Buffer{},
	}
	defer func() {
		if err != nil {
			tio.Close()
		}
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipes: %v", err)
	}
	tio.or, tio.ow = r, w
	r, w, err = os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipes: %v", err)
	}
	tio.er, tio.ew = r, w

	g, _ := errgroup.WithContext(context.TODO())
	tio.g = g
	tio.g.Go(func() error {
		_, err := io.Copy(tio.outBuff, tio.Stdout())
		return err
	})
	tio.g.Go(func() error {
		_, err := io.Copy(tio.errBuff, tio.Stderr())
		return err
	})
	return tio
}

func (t *testIO) Stdin() io.WriteCloser {
	return nil
}

func (t *testIO) Stdout() io.ReadCloser {
	return t.or
}

func (t *testIO) Stderr() io.ReadCloser {
	return t.er
}

func (t *testIO) Set(cmd *exec.Cmd) {
	cmd.Stdout = t.ow
	cmd.Stderr = t.ew
}

func (t *testIO) Close() error {
	var err error
	for _, v := range []*os.File{
		t.ow, t.ew,
		t.or, t.er,
	} {
		if cerr := v.Close(); err == nil {
			err = cerr
		}
	}
	return err
}

func (t *testIO) CloseAfterStart() error {
	t.ow.Close()
	t.ew.Close()
	return nil
}

func (t *testIO) Wait() error {
	return t.g.Wait()
}

func getWindowsImageNameByVersion(t *testing.T, bv int) string {
	switch bv {
	case osversion.RS1:
		return "mcr.microsoft.com/windows/nanoserver:sac2016"
	case osversion.RS3:
		return "mcr.microsoft.com/windows/nanoserver:1709"
	case osversion.RS4:
		return "mcr.microsoft.com/windows/nanoserver:1803"
	case osversion.RS5:
		// testImage = "mcr.microsoft.com/windows/nanoserver:1809"
		return "mcr.microsoft.com/windows/nanoserver/insider:10.0.17763.55"
	default:
		t.Fatalf("unsupported build (%d) for Windows containers", bv)
	}
	// Won't hit because of t.Fatal
	return ""
}

func readPidFile(path string) (int, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return -1, errors.Wrap(err, "failed to read pidfile")
	}
	p, err := strconv.Atoi(string(data))
	if err != nil {
		return -1, errors.Wrap(err, "pidfile failed to parse pid")
	}
	return p, nil
}

func testWindows(t *testing.T, version int, isolated bool) {
	var err error

	// Make the bundle
	bundle := testutilities.CreateTempDir(t)
	defer func() {
		if err == nil {
			os.RemoveAll(bundle)
		} else {
			t.Errorf("additional logs at bundle path: %v", bundle)
		}
	}()
	scratch := testutilities.CreateTempDir(t)
	defer func() {
		vhd.DetachVhd(filepath.Join(scratch, "sandbox.vhdx"))
		os.RemoveAll(scratch)
	}()

	// Generate the Spec
	g, err := generate.New("windows")
	if err != nil {
		t.Errorf("failed to generate Windows config with error: %v", err)
		return
	}
	g.SetProcessArgs([]string{"cmd", "/c", "echo Hello World!"})
	if isolated {
		g.SetWindowsHypervUntilityVMPath("")
	}
	g.Config.Windows.Network = nil

	// Get the LayerFolders
	imageName := getWindowsImageNameByVersion(t, version)
	layers := testutilities.LayerFolders(t, imageName)
	for _, layer := range layers {
		g.AddWindowsLayerFolders(layer)
	}
	g.AddWindowsLayerFolders(scratch)

	cf, err := os.Create(filepath.Join(bundle, "config.json"))
	if err != nil {
		t.Errorf("failed to create config.json with error: %v", err)
		return
	}
	err = json.NewEncoder(cf).Encode(g.Config)
	if err != nil {
		cf.Close()
		t.Errorf("failed to encode config.json with error: %v", err)
		return
	}
	cf.Close()

	// Create the Argon, Xenon, or UVM
	ctx := context.TODO()
	rhcs := runhcs.Runhcs{
		Debug: true,
	}
	tio := newTestIO(t)
	defer func() {
		if err != nil {
			t.Errorf("additional info stdout: '%v', stderr: '%v'", tio.outBuff.String(), tio.errBuff.String())
		}
	}()
	defer func() {
		tio.Close()
	}()
	copts := &runhcs.CreateOpts{
		IO:      tio,
		PidFile: filepath.Join(bundle, "pid-file.txt"),
		ShimLog: filepath.Join(bundle, "shim-log.txt"),
	}
	if isolated {
		copts.VMLog = filepath.Join(bundle, "vm-log.txt")
	}
	err = rhcs.Create(ctx, t.Name(), bundle, copts)
	if err != nil {
		t.Errorf("failed to create container with error: %v", err)
		return
	}
	defer func() {
		rhcs.Delete(ctx, t.Name(), &runhcs.DeleteOpts{Force: true})
	}()

	// Find the shim/vmshim process and begin exit wait
	pid, err := readPidFile(copts.PidFile)
	if err != nil {
		t.Errorf("failed to read pidfile with error: %v", err)
		return
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		t.Errorf("failed to find container process by pid: %d, with error: %v", pid, err)
		return
	}

	// Start the container
	err = rhcs.Start(ctx, t.Name())
	if err != nil {
		t.Errorf("failed to start container with error: %v", err)
		return
	}
	defer func() {
		if err != nil {
			rhcs.Kill(ctx, t.Name(), "CtrlC")
		}
	}()

	// Wait for process exit, verify the exited state
	var exitStatus int
	_, eerr := p.Wait()
	if eerr != nil {
		if exitErr, ok := eerr.(*exec.ExitError); ok {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitStatus = ws.ExitStatus()
			}
		}
	}
	if exitStatus != 0 {
		err = eerr
		t.Errorf("container process failed with exit status: %d", exitStatus)
		return
	}

	// Wait for the relay to exit
	tio.Wait()
	outString := tio.outBuff.String()
	if outString != "Hello World!\r\n" {
		t.Errorf("stdout expected: 'Hello World!', got: '%v'", outString)
	}

	errString := tio.errBuff.String()
	if errString != "" {
		t.Errorf("stderr expected: '', got: '%v'", errString)
	}
}

func testWindowsPod(t *testing.T, version int, isolated bool) {
	t.Skip("not implemented")
}

func testLCOW(t *testing.T) {
	t.Skip("not implemented")
}

func testLCOWPod(t *testing.T) {
	t.Skip("not implemented")
}

func Test_RS1_Argon(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS1)

	testWindows(t, osversion.RS1, false)
}

func Test_RS1_Xenon(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS1)

	testWindows(t, osversion.RS1, true)
}

func Test_RS3_Argon(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS3)

	testWindows(t, osversion.RS3, false)
}

func Test_RS3_Xenon(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS3)

	guests := []int{osversion.RS1, osversion.RS3}
	for _, g := range guests {
		testWindows(t, g, true)
	}
}

func Test_RS4_Argon(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS4)

	testWindows(t, osversion.RS4, false)
}

func Test_RS4_Xenon(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS4)

	guests := []int{osversion.RS1, osversion.RS3, osversion.RS4}
	for _, g := range guests {
		testWindows(t, g, true)
	}
}

func Test_RS5_Argon(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS5)

	testWindows(t, osversion.RS5, false)
}

func Test_RS5_ArgonPods(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS5)

	testWindowsPod(t, osversion.RS5, false)
}

func Test_RS5_UVMAndContainer(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS5)

	guests := []int{osversion.RS1, osversion.RS3, osversion.RS4, osversion.RS5}
	for _, g := range guests {
		testWindows(t, g, true)
	}
}

func Test_RS5_UVMPods(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS5)

	testWindowsPod(t, osversion.RS5, true)
}

func Test_RS5_LCOW(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS5)

	testLCOW(t)
}

func Test_RS5_LCOW_UVMPods(t *testing.T) {
	testutilities.RequiresExactBuild(t, osversion.RS5)

	testLCOWPod(t)
}
