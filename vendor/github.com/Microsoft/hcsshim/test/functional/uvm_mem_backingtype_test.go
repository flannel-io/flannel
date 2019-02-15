// +build functional uvmmem

package functional

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/osversion"
	testutilities "github.com/Microsoft/hcsshim/test/functional/utilities"
	"github.com/sirupsen/logrus"
)

func runMemStartLCOWTest(t *testing.T, opts *uvm.OptionsLCOW) {
	u := testutilities.CreateLCOWUVMFromOpts(t, opts)
	u.Close()
}

func runMemStartWCOWTest(t *testing.T, opts *uvm.OptionsWCOW) {
	u, _, scratchDir := testutilities.CreateWCOWUVMFromOptsWithImage(t, opts, "microsoft/nanoserver")
	defer os.RemoveAll(scratchDir)
	u.Close()
}

func runMemTests(t *testing.T, os string) {
	type testCase struct {
		allowOvercommit      bool
		enableDeferredCommit bool
	}

	testCases := []testCase{
		{allowOvercommit: true, enableDeferredCommit: false},  // Explicit default - Virtual
		{allowOvercommit: true, enableDeferredCommit: true},   // Virtual Deferred
		{allowOvercommit: false, enableDeferredCommit: false}, // Physical
	}

	for _, bt := range testCases {
		if os == "windows" {
			wopts := uvm.NewDefaultOptionsWCOW(t.Name(), "")
			wopts.MemorySizeInMB = 512
			wopts.AllowOvercommit = bt.allowOvercommit
			wopts.EnableDeferredCommit = bt.enableDeferredCommit
			runMemStartWCOWTest(t, wopts)
		} else {
			lopts := uvm.NewDefaultOptionsLCOW(t.Name(), "")
			lopts.MemorySizeInMB = 512
			lopts.AllowOvercommit = bt.allowOvercommit
			lopts.EnableDeferredCommit = bt.enableDeferredCommit
			runMemStartLCOWTest(t, lopts)
		}
	}
}

func TestMemBackingTypeWCOW(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	runMemTests(t, "windows")
}

func TestMemBackingTypeLCOW(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	runMemTests(t, "linux")
}

func runBenchMemStartTest(b *testing.B, opts *uvm.OptionsLCOW) {
	// Cant use testutilities here because its `testing.B` not `testing.T`
	u, err := uvm.CreateLCOW(opts)
	if err != nil {
		b.Fatal(err)
	}
	defer u.Close()
	if err := u.Start(); err != nil {
		b.Fatal(err)
	}
}

func runBenchMemStartLcowTest(b *testing.B, allowOvercommit bool, enableDeferredCommit bool) {
	for i := 0; i < b.N; i++ {
		opts := uvm.NewDefaultOptionsLCOW(b.Name(), "")
		opts.MemorySizeInMB = 512
		opts.AllowOvercommit = allowOvercommit
		opts.EnableDeferredCommit = enableDeferredCommit
		runBenchMemStartTest(b, opts)
	}
}

func BenchmarkMemBackingTypeVirtualLCOW(b *testing.B) {
	//testutilities.RequiresBuild(t, osversion.RS5)
	logrus.SetOutput(ioutil.Discard)

	runBenchMemStartLcowTest(b, true, false)
}

func BenchmarkMemBackingTypeVirtualDeferredLCOW(b *testing.B) {
	//testutilities.RequiresBuild(t, osversion.RS5)
	logrus.SetOutput(ioutil.Discard)

	runBenchMemStartLcowTest(b, true, true)
}

func BenchmarkMemBackingTypePhyscialLCOW(b *testing.B) {
	//testutilities.RequiresBuild(t, osversion.RS5)
	logrus.SetOutput(ioutil.Discard)

	runBenchMemStartLcowTest(b, false, false)
}
