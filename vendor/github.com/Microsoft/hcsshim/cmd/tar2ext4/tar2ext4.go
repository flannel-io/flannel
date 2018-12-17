package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/Microsoft/hcsshim/ext4/tar2ext4"
)

var (
	input      = flag.String("i", "", "input file")
	output     = flag.String("o", "", "output file")
	overlay    = flag.Bool("overlay", false, "produce overlayfs-compatible layer image")
	vhd        = flag.Bool("vhd", false, "add a VHD footer to the end of the image")
	inlineData = flag.Bool("inline", false, "write small file data into the inode; not compatible with DAX")
)

func main() {
	flag.Parse()
	if flag.NArg() != 0 || len(*output) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	err := func() (err error) {
		in := os.Stdin
		if *input != "" {
			in, err = os.Open(*input)
			if err != nil {
				return err
			}
		}
		out, err := os.Create(*output)
		if err != nil {
			return err
		}

		var opts []tar2ext4.Option
		if *overlay {
			opts = append(opts, tar2ext4.ConvertWhiteout)
		}
		if *vhd {
			opts = append(opts, tar2ext4.AppendVhdFooter)
		}
		if *inlineData {
			opts = append(opts, tar2ext4.InlineData)
		}
		err = tar2ext4.Convert(in, out, opts...)
		if err != nil {
			return err
		}

		// Exhaust the tar stream.
		io.Copy(ioutil.Discard, in)
		return nil
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
