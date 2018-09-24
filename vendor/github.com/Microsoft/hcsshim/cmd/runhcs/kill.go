package main

import (
	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/urfave/cli"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill sends the specified signal (default: SIGTERM) to the container's init process",
	ArgsUsage: `<container-id> [signal]

Where "<container-id>" is the name for the instance of the container and
"[signal]" is the signal to be sent to the init process.

EXAMPLE:
For example, if the container id is "ubuntu01" the following will send a "KILL"
signal to the init process of the "ubuntu01" container:

       # runhcs kill ubuntu01 KILL`,
	Flags:  []cli.Flag{},
	Before: appargs.Validate(argID, appargs.Optional(appargs.String)),
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		c, err := getContainer(id, true)
		if err != nil {
			return err
		}
		status, err := c.Status()
		if err != nil {
			return err
		}
		if status != containerRunning {
			return errContainerStopped
		}

		sigstr := context.Args().Get(1)
		if sigstr == "" {
			sigstr = "SIGTERM"
		}

		var pid int
		if err := stateKey.Get(id, keyInitPid, &pid); err != nil {
			return err
		}

		p, err := c.hc.OpenProcess(pid)
		if err != nil {
			return err
		}
		defer p.Close()
		return p.Kill() // BUGBUG: should be Signal
	},
}
