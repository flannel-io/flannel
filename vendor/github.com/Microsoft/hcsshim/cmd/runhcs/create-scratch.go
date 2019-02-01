package main

import (
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/Microsoft/hcsshim/internal/lcow"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/osversion"
	gcsclient "github.com/Microsoft/opengcs/client"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var createScratchCommand = cli.Command{
	Name:        "create-scratch",
	Usage:       "creates a scratch vhdx at 'destpath' that is ext4 formatted",
	Description: "Creates a scratch vhdx at 'destpath' that is ext4 formatted",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "destpath",
			Usage: "Required: describes the destination vhd path",
		},
	},
	Before: appargs.Validate(),
	Action: func(context *cli.Context) error {
		dest := context.String("destpath")
		if dest == "" {
			return errors.New("'destpath' is required")
		}

		// If we only have v1 lcow support do it the old way.
		if osversion.Get().Build < osversion.RS5 {
			cfg := gcsclient.Config{
				Options: gcsclient.Options{
					KirdPath:   filepath.Join(os.Getenv("ProgramFiles"), "Linux Containers"),
					KernelFile: "kernel",
					InitrdFile: uvm.InitrdFile,
				},
				Name:              "createscratch-uvm",
				UvmTimeoutSeconds: 5 * 60, // 5 Min
			}

			if err := cfg.StartUtilityVM(); err != nil {
				return errors.Wrapf(err, "failed to start '%s'", cfg.Name)
			}
			defer cfg.Uvm.Terminate()

			if err := cfg.CreateExt4Vhdx(dest, lcow.DefaultScratchSizeGB, ""); err != nil {
				return errors.Wrapf(err, "failed to create ext4vhdx for '%s'", cfg.Name)
			}
		} else {
			opts := uvm.NewDefaultOptionsLCOW("createscratch-uvm", context.GlobalString("owner"))
			convertUVM, err := uvm.CreateLCOW(opts)
			if err != nil {
				return errors.Wrapf(err, "failed to create '%s'", opts.ID)
			}
			defer convertUVM.Close()
			if err := convertUVM.Start(); err != nil {
				return errors.Wrapf(err, "failed to start '%s'", opts.ID)
			}

			if err := lcow.CreateScratch(convertUVM, dest, lcow.DefaultScratchSizeGB, "", ""); err != nil {
				return errors.Wrapf(err, "failed to create ext4vhdx for '%s'", opts.ID)
			}
		}

		return nil
	},
}
