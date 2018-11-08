package ospath

import (
	"path"
	"path/filepath"
)

// Join joins paths using the target OS's path separator.
func Join(os string, elem ...string) string {
	if os == "windows" {
		return filepath.Join(elem...)
	}
	return path.Join(elem...)
}
