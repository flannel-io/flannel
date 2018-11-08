package uvm

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"syscall"

	"github.com/sirupsen/logrus"
)

const _ERROR_CONNECTION_ABORTED syscall.Errno = 1236

func forwardGcsLogs(l net.Listener) {
	c, err := l.Accept()
	l.Close()
	if err != nil {
		logrus.Error("accepting log socket: ", err)
		return
	}
	j := json.NewDecoder(c)
	logger := logrus.StandardLogger()
	for {
		e := logrus.Entry{Logger: logger}
		err = j.Decode(&e.Data)
		if err == io.EOF || err == _ERROR_CONNECTION_ABORTED {
			break
		}
		if err != nil {
			// Something went wrong. Read the rest of the data as a single
			// string and log it at once -- it's probably a GCS panic stack.
			logrus.Error("gcs log read: ", err)
			rest, _ := ioutil.ReadAll(io.MultiReader(j.Buffered(), c))
			if len(rest) != 0 {
				logrus.Error("gcs stderr: ", string(rest))
			}
			break
		}
		msg := e.Data["msg"]
		delete(e.Data, "msg")
		lvl := e.Data["level"]
		delete(e.Data, "level")
		e.Data["vm.time"] = e.Data["time"]
		delete(e.Data, "time")
		switch lvl {
		case "debug":
			e.Debug(msg)
		case "info":
			e.Info(msg)
		case "warning":
			e.Warning(msg)
		case "error", "fatal":
			e.Error(msg)
		default:
			e.Info(msg)
		}
	}
}

// Start synchronously starts the utility VM.
func (uvm *UtilityVM) Start() error {
	if uvm.gcslog != nil {
		go forwardGcsLogs(uvm.gcslog)
		uvm.gcslog = nil
	}
	return uvm.hcsSystem.Start()
}
