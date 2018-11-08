package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/urfave/cli"
)

var mountCommand = cli.Command{
	Name:      "mount",
	Usage:     "mounts a scratch",
	ArgsUsage: "<scratch path>",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "layer, l",
			Usage: "paths to the parent layers for this layer",
		},
	},
	Action: func(context *cli.Context) (err error) {
		if context.NArg() != 1 {
			return errors.New("invalid usage")
		}
		path, err := filepath.Abs(context.Args().First())
		if err != nil {
			return err
		}

		layers, err := normalizeLayers(context.StringSlice("layer"), true)
		if err != nil {
			return err
		}

		err = hcsshim.ActivateLayer(driverInfo, path)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				hcsshim.DeactivateLayer(driverInfo, path)
			}
		}()

		err = hcsshim.PrepareLayer(driverInfo, path, layers)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				hcsshim.UnprepareLayer(driverInfo, path)
			}
		}()

		mountPath, err := hcsshim.GetLayerMountPath(driverInfo, path)
		if err != nil {
			return err
		}
		_, err = fmt.Println(mountPath)
		return err
	},
}

var unmountCommand = cli.Command{
	Name:      "unmount",
	Usage:     "unmounts a scratch",
	ArgsUsage: "<layer path>",
	Before:    appargs.Validate(appargs.NonEmptyString),
	Action: func(context *cli.Context) (err error) {
		path, err := filepath.Abs(context.Args().First())
		if err != nil {
			return err
		}

		err = hcsshim.UnprepareLayer(driverInfo, path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		err = hcsshim.DeactivateLayer(driverInfo, path)
		if err != nil {
			return err
		}
		return nil
	},
}
