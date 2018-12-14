package main

import (
	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/urfave/cli"
)

var createRunFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "bundle, b",
		Value: "",
		Usage: `path to the root of the bundle directory, defaults to the current directory`,
	},
	cli.StringFlag{
		Name:  "pid-file",
		Value: "",
		Usage: "specify the file to write the process id to",
	},
	cli.StringFlag{
		Name:  "shim-log",
		Value: "",
		Usage: `path to the log file or named pipe (e.g. \\.\pipe\ProtectedPrefix\Administrators\runhcs-<container-id>-shim-log) for the launched shim process`,
	},
	cli.StringFlag{
		Name:  "vm-log",
		Value: "",
		Usage: `path to the log file or named pipe (e.g. \\.\pipe\ProtectedPrefix\Administrators\runhcs-<container-id>-vm-log) for the launched VM shim process`,
	},
	cli.StringFlag{
		Name:  "vm-console",
		Value: "",
		Usage: `path to the pipe for the VM's console (e.g. \\.\pipe\debugpipe)`,
	},
	cli.StringFlag{
		Name:  "host",
		Value: "",
		Usage: "host container whose VM this container should run in",
	},
}

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The create command creates an instance of a container for a bundle. The bundle
is a directory with a specification file named "` + specConfig + `" and a root
filesystem.

The specification file includes an args parameter. The args parameter is used
to specify command(s) that get run when the container is started. To change the
command(s) that get executed on start, edit the args parameter of the spec. See
"runc spec --help" for more explanation.`,
	Flags:  append(createRunFlags),
	Before: appargs.Validate(argID),
	Action: func(context *cli.Context) error {
		cfg, err := containerConfigFromContext(context)
		if err != nil {
			return err
		}
		_, err = createContainer(cfg)
		if err != nil {
			return err
		}
		return nil
	},
}

func containerConfigFromContext(context *cli.Context) (*containerConfig, error) {
	id := context.Args().First()
	pidFile, err := absPathOrEmpty(context.String("pid-file"))
	if err != nil {
		return nil, err
	}
	shimLog, err := absPathOrEmpty(context.String("shim-log"))
	if err != nil {
		return nil, err
	}
	vmLog, err := absPathOrEmpty(context.String("vm-log"))
	if err != nil {
		return nil, err
	}
	spec, err := setupSpec(context)
	if err != nil {
		return nil, err
	}
	return &containerConfig{
		ID:            id,
		Owner:         context.GlobalString("owner"),
		PidFile:       pidFile,
		ShimLogFile:   shimLog,
		VMLogFile:     vmLog,
		VMConsolePipe: context.String("vm-console"),
		Spec:          spec,
		HostID:        context.String("host"),
	}, nil
}
