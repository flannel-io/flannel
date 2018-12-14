package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/Microsoft/hcsshim/internal/runhcs"
)

var argID = appargs.NonEmptyString

func absPathOrEmpty(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, runhcs.SafePipePrefix) {
		if len(path) > len(runhcs.SafePipePrefix) {
			return runhcs.SafePipePath(path[len(runhcs.SafePipePrefix):]), nil
		}
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

func closeWritePipe(pipe net.Conn) error {
	return pipe.(interface {
		CloseWrite() error
	}).CloseWrite()
}
