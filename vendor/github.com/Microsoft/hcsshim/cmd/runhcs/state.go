package main

import (
	"encoding/json"
	"os"

	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/Microsoft/hcsshim/internal/runhcs"
	"github.com/urfave/cli"
)

var stateCommand = cli.Command{
	Name:  "state",
	Usage: "output the state of a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container.`,
	Description: `The state command outputs current state information for the
instance of a container.`,
	Before: appargs.Validate(argID),
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		c, err := getContainer(id, false)
		if err != nil {
			return err
		}
		defer c.Close()
		status, err := c.Status()
		if err != nil {
			return err
		}
		cs := runhcs.ContainerState{
			Version:        c.Spec.Version,
			ID:             c.ID,
			InitProcessPid: c.ShimPid,
			Status:         string(status),
			Bundle:         c.Bundle,
			Rootfs:         c.Rootfs,
			Created:        c.Created,
			Annotations:    c.Spec.Annotations,
		}
		data, err := json.MarshalIndent(cs, "", "  ")
		if err != nil {
			return err
		}
		os.Stdout.Write(data)
		return nil
	},
}
