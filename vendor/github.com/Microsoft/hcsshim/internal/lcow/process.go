package lcow

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Microsoft/hcsshim/internal/copywithtimeout"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/schema2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// ByteCounts are the number of bytes copied to/from standard handles. Note
// this is int64 rather than uint64 to match the golang io.Copy() signature.
type ByteCounts struct {
	In  int64
	Out int64
	Err int64
}

// ProcessOptions are the set of options which are passed to CreateProcessEx() to
// create a utility vm.
type ProcessOptions struct {
	HCSSystem         *hcs.System
	Process           *specs.Process
	Stdin             io.Reader     // Optional reader for sending on to the processes stdin stream
	Stdout            io.Writer     // Optional writer for returning the processes stdout stream
	Stderr            io.Writer     // Optional writer for returning the processes stderr stream
	CopyTimeout       time.Duration // Timeout for the copy
	CreateInUtilityVm bool          // If the compute system is a utility VM
	ByteCounts        ByteCounts    // How much data to copy on each stream if they are supplied. 0 means to io.EOF.
}

// CreateProcess creates a process either in an LCOW utility VM, or for starting
// the init process. TODO: Potentially extend for exec'd processes.
//
// It's essentially a glorified wrapper around hcs.ComputeSystem CreateProcess used
// for internal purposes.
//
// This is used on LCOW to run processes for remote filesystem commands, utilities,
// and debugging.
//
// It optional performs IO copies with timeout between the pipes provided as input,
// and the pipes in the process.
//
// In the ProcessOptions structure, if byte-counts are non-zero, a maximum of those
// bytes are copied to the appropriate standard IO reader/writer. When zero,
// it copies until EOF. It also returns byte-counts indicating how much data
// was sent/received from the process.
//
// It is the responsibility of the caller to call Close() on the process returned.

func CreateProcess(opts *ProcessOptions) (*hcs.Process, *ByteCounts, error) {

	var environment = make(map[string]string)
	copiedByteCounts := &ByteCounts{}

	if opts == nil {
		return nil, nil, fmt.Errorf("no options supplied")
	}

	if opts.HCSSystem == nil {
		return nil, nil, fmt.Errorf("no HCS system supplied")
	}

	if opts.CreateInUtilityVm && opts.Process == nil {
		return nil, nil, fmt.Errorf("process must be supplied for UVM process")
	}

	// Don't pass a process in if this is an LCOW container. This will start the init process.
	if opts.Process != nil {
		for _, v := range opts.Process.Env {
			s := strings.SplitN(v, "=", 2)
			if len(s) == 2 && len(s[1]) > 0 {
				environment[s[0]] = s[1]
			}
		}
		if _, ok := environment["PATH"]; !ok {
			environment["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:"
		}
	}

	processConfig := &ProcessParameters{
		ProcessParameters: hcsschema.ProcessParameters{
			CreateStdInPipe:  (opts.Stdin != nil),
			CreateStdOutPipe: (opts.Stdout != nil),
			CreateStdErrPipe: (opts.Stderr != nil),
			EmulateConsole:   false,
		},
		CreateInUtilityVm: opts.CreateInUtilityVm,
	}

	if opts.Process != nil {
		processConfig.Environment = environment
		processConfig.CommandLine = strings.Join(opts.Process.Args, " ")
		processConfig.WorkingDirectory = opts.Process.Cwd
		if processConfig.WorkingDirectory == "" {
			processConfig.WorkingDirectory = `/`
		}
	}

	proc, err := opts.HCSSystem.CreateProcess(processConfig)
	if err != nil {
		logrus.Debugf("failed to create process: %s", err)
		return nil, nil, err
	}

	processStdin, processStdout, processStderr, err := proc.Stdio()
	if err != nil {
		proc.Kill() // Should this have a timeout?
		proc.Close()
		return nil, nil, fmt.Errorf("failed to get stdio pipes for process %+v: %s", processConfig, err)
	}

	// Send the data into the process's stdin
	if opts.Stdin != nil {
		if copiedByteCounts.In, err = copywithtimeout.Copy(processStdin,
			opts.Stdin,
			opts.ByteCounts.In,
			"stdin",
			opts.CopyTimeout); err != nil {
			return nil, nil, err
		}

		// Don't need stdin now we've sent everything. This signals GCS that we are finished sending data.
		if err := proc.CloseStdin(); err != nil && !hcs.IsNotExist(err) && !hcs.IsAlreadyClosed(err) {
			// This error will occur if the compute system is currently shutting down
			if perr, ok := err.(*hcs.ProcessError); ok && perr.Err != hcs.ErrVmcomputeOperationInvalidState {
				return nil, nil, err
			}
		}
	}

	// Copy the data back from stdout
	if opts.Stdout != nil {
		// Copy the data over to the writer.
		if copiedByteCounts.Out, err = copywithtimeout.Copy(opts.Stdout,
			processStdout,
			opts.ByteCounts.Out,
			"stdout",
			opts.CopyTimeout); err != nil {
			return nil, nil, err
		}
	}

	// Copy the data back from stderr
	if opts.Stderr != nil {
		// Copy the data over to the writer.
		if copiedByteCounts.Err, err = copywithtimeout.Copy(opts.Stderr,
			processStderr,
			opts.ByteCounts.Err,
			"stderr",
			opts.CopyTimeout); err != nil {
			return nil, nil, err
		}
	}
	return proc, copiedByteCounts, nil
}
