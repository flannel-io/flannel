package ip

import (
	"os/exec"
	"syscall"
)

type IPTables struct {
	path string
}

func NewIPTables() (*IPTables, error) {
	path, err := exec.LookPath("iptables")
	if err != nil {
		return nil, err
	}

	return &IPTables{path}, nil
}

func (ipt *IPTables) Exists(table string, args ...string) (bool, error) {
	cmd := append([]string{"-t", table, "-C"}, args...)
	err := exec.Command(ipt.path, cmd...).Run()

	switch {
	case err == nil:
		return true, nil
	case err.(*exec.ExitError).Sys().(syscall.WaitStatus).ExitStatus() == 1:
		return false, nil
	default:
		return false, err
	}
}

func (ipt *IPTables) Append(table string, args ...string) error {
	cmd := append([]string{"-t", table, "-A"}, args...)
	return exec.Command(ipt.path, cmd...).Run()
}

// AppendUnique acts like Append except that it won't add a duplicate
func (ipt *IPTables) AppendUnique(table string, args ...string) error {
	exists, err := ipt.Exists(table, args...)
	if err != nil {
		return err
	}

	if !exists {
		return ipt.Append(table, args...)
	}

	return nil
}

func (ipt *IPTables) ClearChain(table, chain string) error {
	cmd := append([]string{"-t", table, "-N", chain})
	err := exec.Command(ipt.path, cmd...).Run()

	switch {
	case err == nil:
		return nil
	case err.(*exec.ExitError).Sys().(syscall.WaitStatus).ExitStatus() == 1:
		// chain already exists. Flush (clear) it.
		cmd := append([]string{"-t", table, "-F", chain})
		return exec.Command(ipt.path, cmd...).Run()
	default:
		return err
	}
}
