package main

import (
	"errors"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name:  "start",
	Usage: "executes the user defined process in a created container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The start command executes the user defined process in a created container.`,
	Before:      appargs.Validate(argID),
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		container, err := getContainer(id, false)
		if err != nil {
			return err
		}
		defer container.Close()
		status, err := container.Status()
		if err != nil {
			return err
		}
		switch status {
		case containerCreated:
			return container.Exec()
		case containerStopped:
			return errors.New("cannot start a container that has stopped")
		case containerRunning:
			return errors.New("cannot start an already running container")
		default:
			return fmt.Errorf("cannot start a container in the '%s' state", status)
		}
	},
}
