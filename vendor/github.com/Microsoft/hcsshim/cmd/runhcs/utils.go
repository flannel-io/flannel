package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim/internal/appargs"
)

var shimSuccess = []byte{0, 'O', 'K', 0}

var argID = appargs.NonEmptyString

func absPathOrEmpty(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	return filepath.Abs(path)
}

// createPidFile creates a file with the processes pid inside it atomically
// it creates a temp file with the paths filename + '.' infront of it
// then renames the file
func createPidFile(path string, pid int) error {
	var (
		tmpDir  = filepath.Dir(path)
		tmpName = filepath.Join(tmpDir, fmt.Sprintf(".%s", filepath.Base(path)))
	)
	f, err := os.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d", pid)
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func safePipePath(name string) string {
	// Use a pipe in the Administrators protected prefixed to prevent malicious
	// squatting.
	return `\\.\pipe\ProtectedPrefix\Administrators\` + url.PathEscape(name)
}

func closeWritePipe(pipe net.Conn) error {
	return pipe.(interface {
		CloseWrite() error
	}).CloseWrite()
}
