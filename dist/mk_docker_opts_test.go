//go:build linux

// Copyright 2026 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package dist_test exercises the mk-docker-opts.sh script as a plain Go test,
// replacing the former mk-docker-opts_tests.sh bash_unit test.
package dist_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// distDir is the path to the dist/ directory, resolved relative to this file
// so the test works regardless of the working directory.
var distDir = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}()

// sampleSubnetEnv is the content of sample_subnet.env used as input.
const sampleSubnetEnv = `FLANNEL_NETWORK=10.1.0.0/16
FLANNEL_SUBNET=10.1.74.1/24
FLANNEL_MTU=1472
FLANNEL_IPMASQ=false
`

// mkDockerOptsCase describes one invocation of mk-docker-opts.sh.
type mkDockerOptsCase struct {
	name  string
	flags []string
	want  string
}

var mkDockerOptsCases = []mkDockerOptsCase{
	{
		name:  "default (individual + combined)",
		flags: nil,
		want: `DOCKER_OPT_BIP="--bip=10.1.74.1/24"
DOCKER_OPT_IPMASQ="--ip-masq=true"
DOCKER_OPT_MTU="--mtu=1472"
DOCKER_OPTS=" --bip=10.1.74.1/24 --ip-masq=true --mtu=1472"
`,
	},
	{
		name:  "individual only (-i)",
		flags: []string{"-i"},
		want: `DOCKER_OPT_BIP="--bip=10.1.74.1/24"
DOCKER_OPT_IPMASQ="--ip-masq=true"
DOCKER_OPT_MTU="--mtu=1472"
`,
	},
	{
		name:  "combined only (-c)",
		flags: []string{"-c"},
		want: `DOCKER_OPTS=" --bip=10.1.74.1/24 --ip-masq=true --mtu=1472"
`,
	},
	{
		name:  "custom key (-k CUSTOM_KEY)",
		flags: []string{"-k", "CUSTOM_KEY"},
		want: `DOCKER_OPT_BIP="--bip=10.1.74.1/24"
DOCKER_OPT_IPMASQ="--ip-masq=true"
DOCKER_OPT_MTU="--mtu=1472"
CUSTOM_KEY=" --bip=10.1.74.1/24 --ip-masq=true --mtu=1472"
`,
	},
	{
		name:  "strip ip-masq (-m)",
		flags: []string{"-m"},
		want: `DOCKER_OPT_BIP="--bip=10.1.74.1/24"
DOCKER_OPT_MTU="--mtu=1472"
DOCKER_OPTS=" --bip=10.1.74.1/24 --mtu=1472"
`,
	},
}

func TestMkDockerOpts(t *testing.T) {
	// Create a temp directory for input/output files.
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "subnet.env")
	if err := os.WriteFile(inputFile, []byte(sampleSubnetEnv), 0644); err != nil {
		t.Fatalf("writing subnet.env: %v", err)
	}
	outputFile := filepath.Join(tmpDir, "docker_opts.env")

	script := filepath.Join(distDir, "mk-docker-opts.sh")

	for _, tc := range mkDockerOptsCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Remove output file from previous run.
			if err := os.Remove(outputFile); err != nil && !os.IsNotExist(err) {
				t.Fatalf("removing output file: %v", err)
			}

			args := []string{"-f", inputFile, "-d", outputFile}
			args = append(args, tc.flags...)
			cmd := exec.Command(script, args...)
			cmd.Dir = distDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("mk-docker-opts.sh %v failed: %v\n%s", args, err, out)
			}

			got, err := os.ReadFile(outputFile)
			if err != nil {
				t.Fatalf("reading output file: %v", err)
			}

			if normalise(string(got)) != normalise(tc.want) {
				t.Errorf("output mismatch for %q\ngot:\n%s\nwant:\n%s", tc.name, got, tc.want)
			}
		})
	}
}

// normalise strips blank lines and trailing spaces, matching the original
// bash_unit test's `diff -B -b` comparison.
func normalise(s string) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, " \t")
		if l != "" {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}
