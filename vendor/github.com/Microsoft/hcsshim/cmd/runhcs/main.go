package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Microsoft/hcsshim/internal/regstate"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// Add a manifest to get proper Windows version detection.
//
// goversioninfo can be installed with "go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo"

//go:generate goversioninfo -platform-specific

// version will be populated by the Makefile, read from
// VERSION file of the source code.
var version = ""

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

var stateKey *regstate.Key

var logFormat string

const (
	specConfig = "config.json"
	usage      = `Open Container Initiative runtime for Windows

runhcs is a fork of runc, modified to run containers on Windows with or without Hyper-V isolation.  Like runc, it is a command line client for running applications packaged according to the Open Container Initiative (OCI) format.

runhcs integrates with existing process supervisors to provide a production container runtime environment for applications. It can be used with your existing process monitoring tools and the container will be spawned as a direct child of the process supervisor.

Containers are configured using bundles. A bundle for a container is a directory that includes a specification file named "` + specConfig + `".  Bundle contents will depend on the container type.

To start a new instance of a container:

    # runhcs run [ -b bundle ] <container-id>

Where "<container-id>" is your name for the instance of the container that you are starting. The name you provide for the container instance must be unique on your host. Providing the bundle directory using "-b" is optional. The default value for "bundle" is the current directory.`
)

func main() {
	app := cli.NewApp()
	app.Name = "runhcs"
	app.Usage = usage

	var v []string
	if version != "" {
		v = append(v, version)
	}
	if gitCommit != "" {
		v = append(v, fmt.Sprintf("commit: %s", gitCommit))
	}
	v = append(v, fmt.Sprintf("spec: %s", specs.Version))
	app.Version = strings.Join(v, "\n")

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output for logging",
		},
		cli.StringFlag{
			Name:  "log",
			Value: "nul",
			Usage: "set the log file path where internal debug information is written",
		},
		cli.StringFlag{
			Name:  "log-format",
			Value: "text",
			Usage: "set the format used by logs ('text' (default), or 'json')",
		},
		cli.StringFlag{
			Name:  "owner",
			Value: "runhcs",
			Usage: "compute system owner",
		},
		cli.StringFlag{
			Name:  "root",
			Value: "default",
			Usage: "registry key for storage of container state",
		},
	}
	app.Commands = []cli.Command{
		createCommand,
		createScratchCommand,
		deleteCommand,
		// eventsCommand,
		execCommand,
		killCommand,
		listCommand,
		pauseCommand,
		psCommand,
		resizeTtyCommand,
		resumeCommand,
		runCommand,
		shimCommand,
		startCommand,
		stateCommand,
		tarToVhdCommand,
		// updateCommand,
		vmshimCommand,
	}
	app.Before = func(context *cli.Context) error {
		if context.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		if path := context.GlobalString("log"); path != "" {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0666)
			if err != nil {
				return err
			}
			logrus.SetOutput(f)
		}
		switch logFormat = context.GlobalString("log-format"); logFormat {
		case "text":
			// retain logrus's default.
		case "json":
			logrus.SetFormatter(new(logrus.JSONFormatter))
		default:
			return fmt.Errorf("unknown log-format %q", logFormat)
		}

		var err error
		stateKey, err = regstate.Open(context.GlobalString("root"), false)
		if err != nil {
			return err
		}
		return nil
	}
	// If the command returns an error, cli takes upon itself to print
	// the error on cli.ErrWriter and exit.
	// Use our own writer here to ensure the log gets sent to the right location.
	fatalWriter.Writer = cli.ErrWriter
	cli.ErrWriter = &fatalWriter
	if err := app.Run(os.Args); err != nil {
		logrus.Error(err)
		fmt.Fprintln(cli.ErrWriter, err)
		os.Exit(1)
	}
}

type logErrorWriter struct {
	Writer io.Writer
}

var fatalWriter logErrorWriter

func (f *logErrorWriter) Write(p []byte) (n int, err error) {
	logrus.Error(string(p))
	return f.Writer.Write(p)
}
