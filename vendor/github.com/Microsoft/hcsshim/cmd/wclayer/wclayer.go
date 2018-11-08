package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim"
	"github.com/urfave/cli"
)

// Add a manifest to get proper Windows version detection.
//
// goversioninfo can be installed with "go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo"

//go:generate goversioninfo -platform-specific

var usage = `Windows Container layer utility

wclayer is a command line tool for manipulating Windows Container
storage layers. It can import and export layers from and to OCI format
layer tar files, create new writable layers, and mount and unmount
container images.`

var driverInfo = hcsshim.DriverInfo{}

func main() {
	app := cli.NewApp()
	app.Name = "wclayer"
	app.Commands = []cli.Command{
		createCommand,
		exportCommand,
		importCommand,
		mountCommand,
		removeCommand,
		unmountCommand,
	}
	app.Usage = usage

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func normalizeLayers(il []string, needOne bool) ([]string, error) {
	if needOne && len(il) == 0 {
		return nil, errors.New("at least one read-only layer must be specified")
	}
	ol := make([]string, len(il))
	for i := range il {
		var err error
		ol[i], err = filepath.Abs(il[i])
		if err != nil {
			return nil, err
		}
	}
	return ol, nil
}
