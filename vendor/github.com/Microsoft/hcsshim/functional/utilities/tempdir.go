package testutilities

import (
	"io/ioutil"
	"testing"
)

// CreateTempDir creates a temporary directory
func CreateTempDir(t *testing.T) string {
	tempDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	return tempDir
}
