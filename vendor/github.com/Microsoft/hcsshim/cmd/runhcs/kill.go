package main

import (
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/Microsoft/hcsshim/internal/guestrequest"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/schema1"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/pkg/errors"
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
		defer c.Close()
		status, err := c.Status()
		if err != nil {
			return err
		}
		if status != containerRunning {
			return errContainerStopped
		}

		signalsSupported := false

		// The Signal feature was added in RS5
		if osversion.Get().Build >= osversion.RS5 {
			if c.IsHost || c.HostID != "" {
				var hostID string
				if c.IsHost {
					// This is the LCOW, Pod Sandbox, or Windows Xenon V2 for RS5+
					hostID = vmID(c.ID)
				} else {
					// This is the Nth container in a Pod
					hostID = c.HostID
				}
				uvm, err := hcs.OpenComputeSystem(hostID)
				if err != nil {
					return err
				}
				defer uvm.Close()
				if props, err := uvm.Properties(schema1.PropertyTypeGuestConnection); err == nil &&
					props.GuestConnectionInfo.GuestDefinedCapabilities.SignalProcessSupported {
					signalsSupported = true
				}
			} else if c.Spec.Linux == nil && c.Spec.Windows.HyperV == nil {
				// RS5+ Windows Argon
				signalsSupported = true
			}
		}

		signal := 0
		if signalsSupported {
			signal, err = validateSigstr(context.Args().Get(1), signalsSupported, c.Spec.Linux != nil)
			if err != nil {
				return err
			}
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

		if signalsSupported && (c.Spec.Linux != nil || !c.Spec.Process.Terminal) {
			opts := guestrequest.SignalProcessOptions{
				Signal: signal,
			}
			return p.Signal(opts)
		}

		// Legacy signal issue a kill
		return p.Kill()
	},
}

func validateSigstr(sigstr string, signalsSupported bool, isLcow bool) (int, error) {
	errInvalidSignal := errors.Errorf("invalid signal '%s'", sigstr)

	// All flavors including legacy default to SIGTERM on LCOW CtrlC on Windows
	if sigstr == "" {
		if isLcow {
			return 0xf, nil
		}
		return 0, nil
	}

	sigstr = strings.ToUpper(sigstr)

	if !signalsSupported {
		if isLcow {
			switch sigstr {
			case "15":
				fallthrough
			case "TERM":
				fallthrough
			case "SIGTERM":
				return 0xf, nil
			default:
				return 0, errInvalidSignal
			}
		}
		switch sigstr {
		case "0":
			fallthrough
		case "CTRLC":
			return 0x0, nil
		default:
			return 0, errInvalidSignal
		}
	}

	var sigmap map[string]int
	if isLcow {
		sigmap = signalMapLcow
	} else {
		sigmap = signalMapWindows
	}

	signal, err := strconv.Atoi(sigstr)
	if err != nil {
		// Signal might still match the string value
		for k, v := range sigmap {
			if k == sigstr {
				return v, nil
			}
		}
		return 0, errInvalidSignal
	}

	// Match signal by value
	for _, v := range sigmap {
		if signal == v {
			return signal, nil
		}
	}
	return 0, errInvalidSignal
}
