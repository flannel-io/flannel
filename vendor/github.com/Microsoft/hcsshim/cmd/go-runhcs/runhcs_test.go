// +build runhcs_test

package runhcs

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	runc "github.com/containerd/go-runc"
)

func TestRunhcs_E2E(t *testing.T) {
	rhcs := Runhcs{
		Debug: true,
	}

	// TODO: JTERRY75 use this from assets dynamically
	//dir, err := ioutil.TempDir("", "runhcs-bundle")
	//if err != nil {
	//	t.Fatalf("failed to create tempdir with error: %v", err)
	//}
	//defer os.Remove(dir)
	br := os.Getenv("RUNHCS_TEST_BUNDLE_ROOT")
	if br == "" {
		t.Fatal("You must set %RUNHCS_TEST_BUNDLE_ROOT% to the folder containing the test bundles")
		return
	}
	// TODO: JTERRY75 create this spec dynamically once we can do the layer
	// extraction in some way so we dont need hard coded bundle/config.json's
	dir := filepath.Join(br, "runhcs-tee-test")

	ctx := context.TODO()
	id := "runhcs-e2e-id"

	pio, err := runc.NewPipeIO()
	if err != nil {
		t.Fatalf("failed to create new pipe io with error: %v", err)
	}
	defer pio.Close()

	// Write our expected output
	expected := "Hello go-runhcs-container!"
	inbuff := bytes.NewBufferString(expected)
	outbuff := &bytes.Buffer{}
	errbuff := &bytes.Buffer{}
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		_, err := io.Copy(pio.Stdin(), inbuff)
		if err != nil {
			t.Errorf("failed to copy string to stdin pipe.")
		}
		pio.Stdin().Close()
	}()
	go func() {
		_, err := io.Copy(outbuff, pio.Stdout())
		if err != nil {
			t.Errorf("failed to copy string from stdout pipe")
		}
		wg.Done()
	}()
	go func() {
		_, err := io.Copy(errbuff, pio.Stderr())
		if err != nil {
			t.Errorf("failed to copy string from stderr pipe")
		}
		wg.Done()
	}()

	copts := &CreateOpts{
		IO:      pio,
		PidFile: filepath.Join(dir, "pid-file.txt"),
		ShimLog: filepath.Join(dir, "shim-log.txt"),
		VMLog:   filepath.Join(dir, "vm-log.txt"),
	}
	if err := rhcs.Create(ctx, id, dir, copts); err != nil {
		t.Fatalf("failed to create container with error: %v", err)
	}
	defer func() {
		if err := rhcs.Delete(ctx, id, &DeleteOpts{Force: true}); err != nil {
			t.Fatalf("failed to delete container with error: %v", err)
		}
	}()

	if err := rhcs.Start(ctx, id); err != nil {
		t.Fatalf("failed to start container with error: %v", err)
	}
	wg.Wait()

	outstring := outbuff.String()
	if outstring != expected {
		t.Fatalf("stdout string '%s' != expected '%s'", outstring, expected)
	}
	errstring := errbuff.String()
	if errstring != expected {
		t.Fatalf("stderr string '%s' != expected '%s'", errstring, expected)
	}
}
