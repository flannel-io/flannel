package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "execute new process inside the container",
	ArgsUsage: `<container-id> <command> [command options]  || -p process.json <container-id>

Where "<container-id>" is the name for the instance of the container and
"<command>" is the command to be executed in the container.
"<command>" can't be empty unless a "-p" flag provided.

EXAMPLE:
For example, if the container is configured to run the linux ps command the
following will output a list of processes running in the container:

    	# runhcs exec <container-id> ps`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "cwd",
			Usage: "current working directory in the container",
		},
		cli.StringSliceFlag{
			Name:  "env, e",
			Usage: "set environment variables",
		},
		cli.BoolFlag{
			Name:  "tty, t",
			Usage: "allocate a pseudo-TTY",
		},
		cli.StringFlag{
			Name: "user, u",
		},
		cli.StringFlag{
			Name:  "process, p",
			Usage: "path to the process.json",
		},
		cli.BoolFlag{
			Name:  "detach,d",
			Usage: "detach from the container's process",
		},
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
		cli.StringFlag{
			Name:  "shim-log",
			Value: "",
			Usage: `path to the log file or named pipe (e.g. \\.\pipe\ProtectedPrefix\Administrators\runhcs-<container-id>-<exec-id>-log) for the launched shim process`,
		},
	},
	Before: appargs.Validate(argID, appargs.Rest(appargs.String)),
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		pidFile, err := absPathOrEmpty(context.String("pid-file"))
		if err != nil {
			return err
		}
		shimLog, err := absPathOrEmpty(context.String("shim-log"))
		if err != nil {
			return err
		}
		c, err := getContainer(id, false)
		if err != nil {
			return err
		}
		defer c.Close()
		status, err := c.Status()
		if err != nil {
			return err
		}
		if status != containerRunning {
			return errContainerStopped
		}
		spec, err := getProcessSpec(context, c)
		if err != nil {
			return err
		}
		p, err := startProcessShim(id, pidFile, shimLog, spec)
		if err != nil {
			return err
		}
		if !context.Bool("detach") {
			state, err := p.Wait()
			if err != nil {
				return err
			}
			os.Exit(int(state.Sys().(syscall.WaitStatus).ExitCode))
		}
		return nil
	},
	SkipArgReorder: true,
}

func getProcessSpec(context *cli.Context, c *container) (*specs.Process, error) {
	if path := context.String("process"); path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		var p specs.Process
		if err := json.NewDecoder(f).Decode(&p); err != nil {
			return nil, err
		}
		return &p, validateProcessSpec(&p)
	}

	// process via cli flags
	p := c.Spec.Process

	if len(context.Args()) == 1 {
		return nil, fmt.Errorf("process args cannot be empty")
	}
	p.Args = context.Args()[1:]
	// override the cwd, if passed
	if context.String("cwd") != "" {
		p.Cwd = context.String("cwd")
	}
	// append the passed env variables
	p.Env = append(p.Env, context.StringSlice("env")...)

	// set the tty
	if context.IsSet("tty") {
		p.Terminal = context.Bool("tty")
	}
	// override the user, if passed
	if context.String("user") != "" {
		p.User.Username = context.String("user")
	}
	return p, nil
}

func validateProcessSpec(spec *specs.Process) error {
	if spec.Cwd == "" {
		return fmt.Errorf("Cwd property must not be empty")
	}
	// IsAbs doesnt recognize Unix paths on Windows builds so handle that case
	// here.
	if !filepath.IsAbs(spec.Cwd) && !strings.HasPrefix(spec.Cwd, "/") {
		return fmt.Errorf("Cwd must be an absolute path")
	}
	if len(spec.Args) == 0 {
		return fmt.Errorf("args must not be empty")
	}
	return nil
}
