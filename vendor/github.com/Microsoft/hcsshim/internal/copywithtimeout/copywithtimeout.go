package copywithtimeout

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// logDataByteCount is for an advanced debugging technique to allow
// data read/written to a processes stdio channels hex-dumped to the
// log when running at debug level or higher. It is controlled through
// the environment variable HCSSHIM_LOG_DATA_BYTE_COUNT
var logDataByteCount int64

func init() {
	bytes := os.Getenv("HCSSHIM_LOG_DATA_BYTE_COUNT")
	if len(bytes) > 0 {
		u, err := strconv.ParseUint(bytes, 10, 32)
		if err == nil {
			logDataByteCount = int64(u)
		}
	}
}

// Copy is a wrapper for io.Copy using a timeout duration
func Copy(dst io.Writer, src io.Reader, size int64, context string, timeout time.Duration) (int64, error) {
	logrus.WithFields(logrus.Fields{
		"stdval":  context,
		"size":    size,
		"timeout": timeout,
	}).Debug("hcsshim::copywithtimeout - Begin")

	type resultType struct {
		err   error
		bytes int64
	}

	done := make(chan resultType, 1)
	go func() {
		result := resultType{}
		if logrus.GetLevel() < logrus.DebugLevel || logDataByteCount == 0 {
			result.bytes, result.err = io.Copy(dst, src)
		} else {
			// In advanced debug mode where we log (hexdump format) what is copied
			// up to the number of bytes defined by environment variable
			// HCSSHIM_LOG_DATA_BYTE_COUNT
			var buf bytes.Buffer
			tee := io.TeeReader(src, &buf)
			result.bytes, result.err = io.Copy(dst, tee)
			if result.err == nil {
				size := result.bytes
				if size > logDataByteCount {
					size = logDataByteCount
				}
				if size > 0 {
					bytes := make([]byte, size)
					if _, err := buf.Read(bytes); err == nil {
						logrus.Debugf("hcsshim::copyWithTimeout - Read bytes\n%s", hex.Dump(bytes))
					}
				}
			}
		}
		done <- result
	}()

	var result resultType
	timedout := time.After(timeout)

	select {
	case <-timedout:
		return 0, fmt.Errorf("hcsshim::copyWithTimeout: timed out (%s)", context)
	case result = <-done:
		if result.err != nil && result.err != io.EOF {
			// See https://github.com/golang/go/blob/f3f29d1dea525f48995c1693c609f5e67c046893/src/os/exec/exec_windows.go for a clue as to why we are doing this :)
			if se, ok := result.err.(syscall.Errno); ok {
				const (
					errNoData     = syscall.Errno(232)
					errBrokenPipe = syscall.Errno(109)
				)
				if se == errNoData || se == errBrokenPipe {
					logrus.WithFields(logrus.Fields{
						"stdval":        context,
						logrus.ErrorKey: se,
					}).Debug("hcsshim::copywithtimeout - End")
					return result.bytes, nil
				}
			}
			return 0, fmt.Errorf("hcsshim::copyWithTimeout: error reading: '%s' after %d bytes (%s)", result.err, result.bytes, context)
		}
	}
	logrus.WithFields(logrus.Fields{
		"stdval":       context,
		"copied-bytes": result.bytes,
	}).Debug("hcsshim::copywithtimeout - Completed Successfully")
	return result.bytes, nil
}
