package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"syscall"

	winio "github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func vmID(id string) string {
	return id + "@vm"
}

var vmshimCommand = cli.Command{
	Name:   "vmshim",
	Usage:  `launch a VM and containers inside it (do not call it outside of runhcs)`,
	Hidden: true,
	Flags:  []cli.Flag{},
	Before: appargs.Validate(argID),
	Action: func(context *cli.Context) error {
		logrus.SetOutput(os.Stderr)
		fatalWriter.Writer = os.Stdout

		pipePath := context.Args().First()

		optsj, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		os.Stdin.Close()

		opts := &uvm.UVMOptions{}
		err = json.Unmarshal(optsj, opts)
		if err != nil {
			return err
		}

		// Listen on the named pipe associated with this VM.
		l, err := winio.ListenPipe(pipePath, &winio.PipeConfig{MessageMode: true})
		if err != nil {
			return err
		}

		vm, err := startVM(opts)
		if err != nil {
			return err
		}

		// Asynchronously wait for the VM to exit.
		exitCh := make(chan error)
		go func() {
			exitCh <- vm.Wait()
		}()

		defer vm.Terminate()

		// Alert the parent process that initialization has completed
		// successfully.
		os.Stdout.Write(shimSuccess)
		os.Stdout.Close()
		fatalWriter.Writer = ioutil.Discard

		pipeCh := make(chan net.Conn)
		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					logrus.Error(err)
					continue
				}
				pipeCh <- conn
			}
		}()

		for {
			select {
			case <-exitCh:
				return nil
			case pipe := <-pipeCh:
				err = processRequest(vm, pipe)
				if err == nil {
					_, err = pipe.Write(shimSuccess)
					// Wait until the pipe is closed before closing the
					// container so that it is properly handed off to the other
					// process.
					if err == nil {
						err = closeWritePipe(pipe)
					}
					if err == nil {
						ioutil.ReadAll(pipe)
					}
				} else {
					logrus.Error("failed creating container in VM: ", err)
					fmt.Fprintf(pipe, "%v", err)
				}
				pipe.Close()
			}
		}
	},
}

type vmRequestOp string

const (
	opCreateContainer          vmRequestOp = "create"
	opUnmountContainer         vmRequestOp = "unmount"
	opUnmountContainerDiskOnly vmRequestOp = "unmount-disk"
)

type vmRequest struct {
	ID string
	Op vmRequestOp
}

func startVM(opts *uvm.UVMOptions) (*uvm.UtilityVM, error) {
	vm, err := uvm.Create(opts)
	if err != nil {
		return nil, err
	}
	err = vm.Start()
	if err != nil {
		vm.Close()
		return nil, err
	}
	return vm, nil
}

func processRequest(vm *uvm.UtilityVM, pipe net.Conn) error {
	var req vmRequest
	err := json.NewDecoder(pipe).Decode(&req)
	if err != nil {
		return err
	}
	logrus.Debug("received operation ", req.Op, " for ", req.ID)
	c, err := getContainer(req.ID, false)
	if err != nil {
		return err
	}
	defer func() {
		if c != nil {
			c.Close()
		}
	}()
	switch req.Op {
	case opCreateContainer:
		err = createContainerInHost(c, vm)
		if err != nil {
			return err
		}
		c2 := c
		c = nil
		go func() {
			c2.hc.Wait()
			c2.Close()
		}()
		c = nil

	case opUnmountContainer, opUnmountContainerDiskOnly:
		err = c.unmountInHost(vm, req.Op == opUnmountContainer)
		if err != nil {
			return err
		}

	default:
		panic("unknown operation")
	}
	return nil
}

type noVMError struct {
	ID string
}

func (err *noVMError) Error() string {
	return "VM " + err.ID + " cannot be contacted"
}

func (c *container) issueVMRequest(op vmRequestOp) error {
	pipe, err := winio.DialPipe(c.VMPipePath(), nil)
	if err != nil {
		if perr, ok := err.(*os.PathError); ok && perr.Err == syscall.ERROR_FILE_NOT_FOUND {
			return &noVMError{c.HostID}
		}
		return err
	}
	defer pipe.Close()
	req := vmRequest{
		ID: c.ID,
		Op: op,
	}
	err = json.NewEncoder(pipe).Encode(&req)
	if err != nil {
		return err
	}
	err = getErrorFromPipe(pipe, nil)
	if err != nil {
		return err
	}
	return nil
}
