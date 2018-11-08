package main

import (
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/Microsoft/hcsshim/internal/ociwclayer"
	"github.com/urfave/cli"
)

var importCommand = cli.Command{
	Name:  "import",
	Usage: "imports a layer from a tar file",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "layer, l",
			Usage: "paths to the read-only parent layers",
		},
		cli.StringFlag{
			Name:  "input, i",
			Usage: "input layer tar (defaults to stdin)",
		},
	},
	ArgsUsage: "<layer path>",
	Before:    appargs.Validate(appargs.NonEmptyString),
	Action: func(context *cli.Context) (err error) {
		path, err := filepath.Abs(context.Args().First())
		if err != nil {
			return err
		}

		layers, err := normalizeLayers(context.StringSlice("layer"), false)
		if err != nil {
			return err
		}

		fp := context.String("input")
		f := os.Stdin
		if fp != "" {
			f, err = os.Open(fp)
			if err != nil {
				return err
			}
			defer f.Close()
		}
		r, err := addDecompressor(f)
		if err != nil {
			return err
		}
		err = winio.EnableProcessPrivileges([]string{winio.SeBackupPrivilege, winio.SeRestorePrivilege})
		if err != nil {
			return err
		}
		_, err = ociwclayer.ImportLayer(r, path, layers)
		return err
	},
}

func addDecompressor(r io.Reader) (io.Reader, error) {
	b := bufio.NewReader(r)
	hdr, err := b.Peek(3)
	if err != nil {
		return nil, err
	}
	if hdr[0] == 0x1f && hdr[1] == 0x8b && hdr[2] == 8 {
		return gzip.NewReader(b)
	}
	return b, nil
}
