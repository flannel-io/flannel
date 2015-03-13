// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ip

import (
	"bytes"
	"fmt"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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
	checkPresent, err := getIptablesHasCheckCommand()
	if err != nil {
		log.Warningf("Error checking iptables version, assuming version at least 1.4.11: %v\n", err)
		checkPresent = true
	}

	if !checkPresent {
		cmd := append([]string{"-A"}, args...)
		return existsForOldIpTables(table, strings.Join(cmd, " "))
	} else {
		cmd := append([]string{"-t", table, "-C"}, args...)
		err = exec.Command(ipt.path, cmd...).Run()
	}
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

// Checks if iptables has the "-C" flag
func getIptablesHasCheckCommand() (bool, error) {
	vstring, err := getIptablesVersionString()
	if err != nil {
		return false, err
	}

	v1, v2, v3, err := extractIptablesVersion(vstring)
	if err != nil {
		return false, err
	}

	return iptablesHasCheckCommand(v1, v2, v3), nil
}

// getIptablesVersion returns the first three components of the iptables version.
// e.g. "iptables v1.3.66" would return (1, 3, 66, nil)
func extractIptablesVersion(str string) (int, int, int, error) {
	versionMatcher := regexp.MustCompile("v([0-9]+)\\.([0-9]+)\\.([0-9]+)")
	result := versionMatcher.FindStringSubmatch(str)
	if result == nil {
		return 0, 0, 0, fmt.Errorf("no iptables version found in string: %s", str)
	}

	v1, err := strconv.Atoi(result[1])
	if err != nil {
		return 0, 0, 0, err
	}

	v2, err := strconv.Atoi(result[2])
	if err != nil {
		return 0, 0, 0, err
	}

	v3, err := strconv.Atoi(result[3])
	if err != nil {
		return 0, 0, 0, err
	}

	return v1, v2, v3, nil
}

// Runs "iptables --version" to get the version string
func getIptablesVersionString() (string, error) {
	cmd := exec.Command("iptables", "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

// Checks if an iptables version is after 1.4.11, when --check was added
func iptablesHasCheckCommand(v1 int, v2 int, v3 int) bool {
	if v1 > 1 {
		return true
	}
	if v1 == 1 && v2 > 4 {
		return true
	}
	if v1 == 1 && v2 == 4 && v3 >= 11 {
		return true
	}
	return false
}

// Checks if a rule specification exists for a table
func existsForOldIpTables(table string, ruleSpec string) (bool, error) {
	cmd := exec.Command("iptables", "-t", table, "-S")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return false, err
	}
	rules := out.String()
	return strings.Contains(rules, ruleSpec), nil
}
