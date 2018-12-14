package main

import (
	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/urfave/cli"
)

var pauseCommand = cli.Command{
	Name:  "pause",
	Usage: "pause suspends all processes inside the container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
paused. `,
	Description: `The pause command suspends all processes in the instance of the container.

Use runhcs list to identify instances of containers and their current status.`,
	Before: appargs.Validate(argID),
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		container, err := getContainer(id, true)
		if err != nil {
			return err
		}
		defer container.Close()
		if err := container.hc.Pause(); err != nil {
			return err
		}

		return nil
	},
}

var resumeCommand = cli.Command{
	Name:  "resume",
	Usage: "resumes all processes that have been previously paused",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
resumed.`,
	Description: `The resume command resumes all processes in the instance of the container.

Use runhcs list to identify instances of containers and their current status.`,
	Before: appargs.Validate(argID),
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		container, err := getContainer(id, true)
		if err != nil {
			return err
		}
		defer container.Close()
		if err := container.hc.Resume(); err != nil {
			return err
		}

		return nil
	},
}
