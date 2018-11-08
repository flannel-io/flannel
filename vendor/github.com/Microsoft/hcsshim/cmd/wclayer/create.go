package main

import (
	"path/filepath"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "creates a new writable container layer",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "layer, l",
			Usage: "paths to the read-only parent layers",
		},
	},
	ArgsUsage: "<layer path>",
	Before:    appargs.Validate(appargs.NonEmptyString),
	Action: func(context *cli.Context) error {
		path, err := filepath.Abs(context.Args().First())
		if err != nil {
			return err
		}

		layers, err := normalizeLayers(context.StringSlice("layer"), true)
		if err != nil {
			return err
		}

		di := driverInfo
		return hcsshim.CreateScratchLayer(di, path, layers[len(layers)-1], layers)
	},
}
