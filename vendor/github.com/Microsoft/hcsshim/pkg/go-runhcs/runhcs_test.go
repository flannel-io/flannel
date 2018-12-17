package runhcs

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func resetRunhcsPath() {
	runhcsPath = atomic.Value{}
}

func TestGetCommandPath_NoLookPath(t *testing.T) {
	resetRunhcsPath()

	path := getCommandPath()
	if path != "runhcs.exe" {
		t.Fatalf("expected path 'runhcs.exe' got '%s'", path)
	}
	pathi := runhcsPath.Load()
	if pathi == nil {
		t.Fatal("cache state should be set after first query")
	}
	if path != pathi.(string) {
		t.Fatalf("expected: '%s' in cache got '%s'", path, pathi.(string))
	}
}

func TestGetCommandPath_WithLookPath(t *testing.T) {
	resetRunhcsPath()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd with err: %v", err)
	}
	fakePath := filepath.Join(wd, "runhcs.exe")
	f, err := os.Create(fakePath)
	if err != nil {
		t.Fatalf("failed to create fake runhcs.exe in path with err: %v", err)
	}
	f.Close()
	defer os.Remove(fakePath)

	path := getCommandPath()
	if path != fakePath {
		t.Fatalf("expected fake path '%s' got '%s'", fakePath, path)
	}
	pathi := runhcsPath.Load()
	if pathi == nil {
		t.Fatal("cache state should be set after first query")
	}
	if path != pathi.(string) {
		t.Fatalf("expected: '%s' in cache got '%s'", fakePath, pathi.(string))
	}
}

func TestGetCommandPath_WithCache(t *testing.T) {
	resetRunhcsPath()

	value := "this is a test"
	runhcsPath.Store(value)

	path := getCommandPath()
	if path != value {
		t.Fatalf("expected fake cached path: '%s' got '%s'", value, path)
	}
}
