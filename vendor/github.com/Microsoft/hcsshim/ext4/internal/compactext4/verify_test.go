// +build !linux

package compactext4

import "testing"

func verifyTestFile(t *testing.T, mountPath string, tf testFile) {
}

func mountImage(t *testing.T, image string, mountPath string) bool {
	return false
}

func unmountImage(t *testing.T, mountPath string) {
}

func fsck(t *testing.T, image string) {
}
