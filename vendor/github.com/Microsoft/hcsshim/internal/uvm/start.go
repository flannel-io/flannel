package uvm

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"syscall"

	"github.com/sirupsen/logrus"
)

const _ERROR_CONNECTION_ABORTED syscall.Errno = 1236

var _ = (OutputHandler)(parseLogrus)

func parseLogrus(r io.Reader) {
	j := json.NewDecoder(r)
	logger := logrus.StandardLogger()
	for {
		e := logrus.Entry{Logger: logger}
		err := j.Decode(&e.Data)
		if err == io.EOF || err == _ERROR_CONNECTION_ABORTED {
			break
		}
		if err != nil {
			// Something went wrong. Read the rest of the data as a single
			// string and log it at once -- it's probably a GCS panic stack.
			logrus.Error("gcs log read: ", err)
			rest, _ := ioutil.ReadAll(io.MultiReader(j.Buffered(), r))
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

type acceptResult struct {
	c   net.Conn
	err error
}

func processOutput(ctx context.Context, l net.Listener, doneChan chan struct{}, handler OutputHandler) {
	defer close(doneChan)

	ch := make(chan acceptResult)
	go func() {
		c, err := l.Accept()
		ch <- acceptResult{c, err}
	}()

	select {
	case <-ctx.Done():
		l.Close()
		return
	case ar := <-ch:
		c, err := ar.c, ar.err
		l.Close()
		if err != nil {
			logrus.Error("accepting log socket: ", err)
			return
		}
		defer c.Close()

		handler(c)
	}
}

// Start synchronously starts the utility VM.
func (uvm *UtilityVM) Start() error {
	if uvm.outputListener != nil {
		ctx, cancel := context.WithCancel(context.Background())
		go processOutput(ctx, uvm.outputListener, uvm.outputProcessingDone, uvm.outputHandler)
		uvm.outputProcessingCancel = cancel
		uvm.outputListener = nil
	}
	return uvm.hcsSystem.Start()
}
