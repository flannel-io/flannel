package tool

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/thxcode/winnet/pkg/converters"
)

// ExecuteNetsh executes the netsh command
func ExecuteNetsh(args []string) (bool, string, error) {
	if len(args) == 0 {
		return false, "", errors.New("could not execute netsh with an empty args array")
	}

	cmd := exec.Command("netsh", args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return true, converters.UnsafeBytesToString(output), nil
	}

	// netsh uses exit(0) to indicate a success of the operation
	if ps := cmd.ProcessState; ps != nil && ps.Exited() && ps.ExitCode() != 0 {
		return false, converters.UnsafeBytesToString(output), nil
	}

	return false, converters.UnsafeBytesToString(output), fmt.Errorf("failed to execute: netsh %s: %v", strings.Join(args, " "), err)
}
