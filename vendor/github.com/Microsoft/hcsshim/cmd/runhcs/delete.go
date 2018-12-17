package main

import (
	"fmt"
	"os"

	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/Microsoft/hcsshim/internal/regstate"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete any resources held by the container often used with detached container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container.

EXAMPLE:
For example, if the container id is "ubuntu01" and runhcs list currently shows the
status of "ubuntu01" as "stopped" the following will delete resources held for
"ubuntu01" removing "ubuntu01" from the runhcs list of containers:

       # runhcs delete ubuntu01`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Forcibly deletes the container if it is still running (uses SIGKILL)",
		},
	},
	Before: appargs.Validate(argID),
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		force := context.Bool("force")
		container, err := getContainer(id, false)
		if err != nil {
			if _, ok := err.(*regstate.NoStateError); ok {
				if e := stateKey.Remove(id); e != nil {
					fmt.Fprintf(os.Stderr, "remove %s: %v\n", id, e)
				}
				if force {
					return nil
				}
			}
			return err
		}
		defer container.Close()
		s, err := container.Status()
		if err != nil {
			return err
		}

		kill := false
		switch s {
		case containerStopped:
		case containerCreated:
			kill = true
		default:
			if !force {
				return fmt.Errorf("cannot delete container %s that is not stopped: %s\n", id, s)
			}
			kill = true
		}

		if kill {
			err = container.Kill()
			if err != nil {
				return err
			}
		}
		return container.Remove()
	},
}
