package main

import (
	"fmt"
	"strconv"

	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/urfave/cli"
)

var resizeTtyCommand = cli.Command{
	Name:      "resize-tty",
	Usage:     "resize-tty updates the terminal size for a container process",
	ArgsUsage: `<container-id> <width> <height>`,
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:  "pid, p",
			Usage: "the process pid (defaults to init pid)",
		},
	},
	Before: appargs.Validate(
		argID,
		appargs.Int(10, 1, 65535),
		appargs.Int(10, 1, 65535),
	),
	Action: func(context *cli.Context) error {
		id := context.Args()[0]
		width, _ := strconv.ParseUint(context.Args()[1], 10, 16)
		height, _ := strconv.ParseUint(context.Args()[2], 10, 16)
		c, err := getContainer(id, true)
		if err != nil {
			return err
		}
		defer c.Close()

		pid := context.Int("pid")
		if pid == 0 {
			if err := stateKey.Get(id, keyInitPid, &pid); err != nil {
				return err
			}
		} else {
			// If a pid was provided map it to its hcs pid.
			if err := stateKey.Get(id, fmt.Sprintf(keyPidMapFmt, pid), &pid); err != nil {
				return err
			}
		}

		p, err := c.hc.OpenProcess(pid)
		if err != nil {
			return err
		}
		defer p.Close()

		return p.ResizeConsole(uint16(width), uint16(height))
	},
}
