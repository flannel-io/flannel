package main

import (
	"io"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/Microsoft/hcsshim/internal/lcow"
	"github.com/Microsoft/hcsshim/internal/osversion"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	gcsclient "github.com/Microsoft/opengcs/client"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var tarToVhdCommand = cli.Command{
	Name:        "tar2vhd",
	Usage:       "converts a tar over stdin to a vhd at 'destpath'",
	Description: "The tar2vhd command converts the tar at ('sourcepath'|stdin) to a vhd at 'destpath'",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "sourcepath",
			Usage: "Optional: describes the path to the tar on disk",
		},
		cli.StringFlag{
			Name:  "scratchpath",
			Usage: "Required: describes the path to the scratch.vhdx file to use for the transformation",
		},
		cli.StringFlag{
			Name:  "destpath",
			Usage: "Required: describes the destination vhd path to write the contents of the tar to on disk",
		},
	},
	Before: appargs.Validate(),
	Action: func(context *cli.Context) error {
		var rdr io.Reader
		if src := context.String("sourcepath"); src != "" {
			// Source is via file path not stdin
			f, err := os.OpenFile(src, os.O_RDONLY, 0)
			if err != nil {
				return errors.Wrapf(err, "failed to open 'sourcepath': '%s'", src)
			}
			defer f.Close()
			rdr = f
		} else {
			rdr = os.Stdin
		}

		scratch := context.String("scratchpath")
		if scratch == "" {
			return errors.New("'scratchpath' is required")
		}

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
					InitrdFile: "initrd.img",
				},
				Name:              "tar2vhd-uvm",
				UvmTimeoutSeconds: 5 * 60, // 5 Min
			}

			if err := cfg.StartUtilityVM(); err != nil {
				return errors.Wrapf(err, "failed to start '%s'", cfg.Name)
			}
			defer cfg.Uvm.Terminate()

			if err := cfg.HotAddVhd(scratch, "/tmp/scratch", false, true); err != nil {
				return errors.Wrapf(err, "failed to mount scratch path: '%s' to '%s'", scratch, cfg.Name)
			}

			n, err := cfg.TarToVhd(dest, rdr)
			if err != nil {
				return errors.Wrapf(err, "failed to convert tar2vhd for '%s'", cfg.Name)
			}

			logrus.Debugf("wrote %v bytes to %s", n, dest)
		} else {
			opts := uvm.UVMOptions{
				ID:              "tar2vhd-uvm",
				OperatingSystem: "linux",
			}
			convertUVM, err := uvm.Create(&opts)
			if err != nil {
				return errors.Wrapf(err, "failed to create '%s'", opts.ID)
			}
			if err := convertUVM.Start(); err != nil {
				return errors.Wrapf(err, "failed to start '%s'", opts.ID)
			}
			defer convertUVM.Terminate()

			if err := wclayer.GrantVmAccess(opts.ID, scratch); err != nil {
				return errors.Wrapf(err, "failed to grant access to scratch path: '%s' to '%s'", scratch, opts.ID)
			}
			if _, _, err := convertUVM.AddSCSI(scratch, "/tmp/scratch"); err != nil {
				return errors.Wrapf(err, "failed to mount scratch path: '%s' to '%s'", scratch, opts.ID)
			}

			n, err := lcow.TarToVhd(convertUVM, dest, rdr)
			if err != nil {
				return errors.Wrapf(err, "failed to convert tar2vhd for '%s'", opts.ID)
			}

			logrus.Debugf("wrote %v bytes to %s", n, dest)
		}

		return nil
	},
}
