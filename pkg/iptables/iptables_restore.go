// Copyright 2015 flannel authors
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
//go:build !windows
// +build !windows

package iptables

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/coreos/go-iptables/iptables"
	log "k8s.io/klog"
)

const (
	ipTablesRestoreCmd  string = "iptables-restore"
	ip6TablesRestoreCmd string = "ip6tables-restore"
	ipTablesCmd         string = "iptables"
	ip6TablesCmd        string = "ip6tables"
)

// IPTablesRestore wrapper for iptables-restore
type IPTablesRestore interface {
	// ApplyWithoutFlush apply without flush chains
	ApplyWithoutFlush(rules IPTablesRestoreRules) error
}

// ipTablesRestore internal type
type ipTablesRestore struct {
	path    string
	proto   iptables.Protocol
	hasWait bool
}

// IPTablesRestoreRules represents iptables-restore table block
type IPTablesRestoreRules map[string][]IPTablesRestoreRuleSpec

// IPTablesRestoreRuleSpec represents one rule spec delimited by space
type IPTablesRestoreRuleSpec []string

// NewIPTablesRestoreWithProtocol build new IPTablesRestore for supplied proto
func NewIPTablesRestoreWithProtocol(protocol iptables.Protocol) (IPTablesRestore, error) {
	cmd := getIptablesRestoreCommand(protocol)
	path, err := exec.LookPath(cmd)
	if err != nil {
		return nil, err
	}
	cmdIptables := getIptablesCommand(protocol)
	pathIptables, err := exec.LookPath(cmdIptables)
	if err != nil {
		return nil, err
	}
	hasWait, err := getIptablesRestoreSupport(pathIptables)
	if err != nil {
		return nil, err
	}

	ipt := ipTablesRestore{
		path:    path,
		proto:   protocol,
		hasWait: hasWait,
	}
	return &ipt, nil
}

// ApplyWithoutFlush apply without flush chains
func (iptr *ipTablesRestore) ApplyWithoutFlush(rules IPTablesRestoreRules) error {
	payload := buildIPTablesRestorePayload(rules)

	log.V(6).Infof("trying to run with payload %s", payload)
	stdout, stderr, err := iptr.runWithOutput([]string{"--noflush"}, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return fmt.Errorf("unable to run iptables-restore (%s, %s): %v", stdout, stderr, err)
	}
	return nil
}

// runWithOutput runs an iptables command with the given arguments,
// writing any stdout output to the given writer
func (iptr *ipTablesRestore) runWithOutput(args []string, stdin io.Reader) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if iptr.hasWait {
		args = append(args, "--wait")
	}

	cmd := exec.Command(iptr.path, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = stdin

	if err := cmd.Run(); err != nil {
		return "", "", err
	}

	return stdout.String(), stderr.String(), nil
}

// buildIPTablesRestorePayload build table/COMMIT payload
func buildIPTablesRestorePayload(tableRules IPTablesRestoreRules) string {
	iptablesRestorePayload := ""
	for table, rules := range tableRules {
		iptablesRestorePayload += "*" + table + "\n"

		for _, lineRule := range rules {
			// as iptables-restore use stdin if "--comment" then protect "the comment"
			size := len(lineRule)
			for i, rule := range lineRule {
				if i > 0 && lineRule[i-1] == "--comment" {
					iptablesRestorePayload += "\"" + rule + "\""
				} else {
					iptablesRestorePayload += rule
				}

				if i < size-1 {
					iptablesRestorePayload += " "
				} else {
					iptablesRestorePayload += "\n"
				}
			}
		}

		iptablesRestorePayload += "COMMIT\n"
	}
	return iptablesRestorePayload
}

// getIptablesRestoreSupport get current iptables-restore support
func getIptablesRestoreSupport(path string) (hasWait bool, err error) {
	version, err := getIptablesRestoreVersionString(path)
	if err != nil {
		return false, err
	}
	v1, v2, v3, err := extractIptablesRestoreVersion(version)
	if err != nil {
		return false, err
	}
	return ipTablesHasWaitSupport(v1, v2, v3), nil
}

// Checks if an iptables-restore version is after 1.6.2, when --wait was added
func ipTablesHasWaitSupport(v1, v2, v3 int) bool {
	if v1 > 1 {
		return true
	}
	if v1 == 1 && v2 > 6 {
		return true
	}
	if v1 == 1 && v2 == 6 && v3 >= 2 {
		return true
	}
	return false
}

// extractIptablesRestoreVersion returns the first three components of the iptables-restore version
// e.g. "iptables-restore v1.3.66" would return (1, 3, 66, nil)
func extractIptablesRestoreVersion(str string) (int, int, int, error) {
	versionMatcher := regexp.MustCompile(`v([0-9]+)\.([0-9]+)\.([0-9]+)`)
	result := versionMatcher.FindStringSubmatch(str)
	if result == nil {
		return 0, 0, 0, fmt.Errorf("no iptables-restore version found in string: %s", str)
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

// getIptablesRestoreVersionString run iptables-restore --version
func getIptablesRestoreVersionString(path string) (string, error) {
	cmd := exec.Command(path, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("unable to find iptables-restore version: %v", err)
	}
	return out.String(), nil
}

// getIptablesRestoreCommand returns the correct command for the given proto, either "iptables-restore" or "ip6tables-restore".
func getIptablesRestoreCommand(proto iptables.Protocol) string {
	if proto == iptables.ProtocolIPv6 {
		return ip6TablesRestoreCmd
	}
	return ipTablesRestoreCmd
}

// getIptablesCommand returns the correct command for the given proto, either "iptables" or "ip6tables".
func getIptablesCommand(proto iptables.Protocol) string {
	if proto == iptables.ProtocolIPv6 {
		return ip6TablesCmd
	}
	return ipTablesCmd
}
