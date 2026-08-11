//go:build functional

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
package functional

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
)

// docker0IP returns the IP address of the docker0 bridge on the host,
// equivalent to `ip -o -f inet addr show docker0 | grep -Po 'inet \K[\d.]+'`.
func docker0IP() (string, error) {
	iface, err := net.InterfaceByName("docker0")
	if err != nil {
		return "", fmt.Errorf("looking up docker0: %w", err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("getting docker0 addresses: %w", err)
	}
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.String())
		if err != nil {
			continue
		}
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address on docker0")
}

// dockerRm removes a container by name, ignoring errors (container may not exist).
func dockerRm(name string) {
	_ = exec.Command("docker", "rm", "-f", name).Run()
}

// dockerRun runs `docker run` with the given arguments and returns an error if
// the command fails. stdout+stderr are discarded (containers run detached).
func dockerRun(args ...string) (string, error) {
	cmd := exec.Command("docker", append([]string{"run"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker run %v: %w\n%s", args, err, out)
	}
	return string(out), nil
}

// dockerExec runs `docker exec` in the named container and returns combined
// output.  privileged enables `--privileged`.
func dockerExec(container string, privileged bool, args ...string) (string, error) {
	dArgs := []string{"exec"}
	if privileged {
		dArgs = append(dArgs, "--privileged")
	}
	dArgs = append(dArgs, container)
	dArgs = append(dArgs, args...)
	cmd := exec.Command("docker", dArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// dockerExecI runs `docker exec -i` (stdin attached), piping the provided stdin
// content and returning combined output.
func dockerExecI(container string, stdin string, args ...string) (string, error) {
	dArgs := append([]string{"exec", "-i", container}, args...)
	cmd := exec.Command("docker", dArgs...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// dockerLogs returns the logs from a named container.
func dockerLogs(name string) string {
	out, _ := exec.Command("docker", "logs", name).CombinedOutput()
	return string(out)
}

// dockerInspectStatus returns the Status field of a container.
func dockerInspectStatus(name string) string {
	out, _ := exec.Command("docker", "inspect", "--format={{.State.Status}}", name).Output()
	return strings.TrimSpace(string(out))
}

// etcdctl runs an etcdctl command via docker against the given endpoint and
// certs directory, returning combined output.
func etcdctl(endpoint, certsDir string, args ...string) (string, error) {
	dArgs := []string{
		"run", "--rm",
		"-e", "ETCDCTL_API=3",
		"-v", certsDir + ":/certs",
		etcdctlImg, "etcdctl",
		"--endpoints=" + endpoint,
		"--cacert=/certs/ca.pem",
		"--cert=/certs/client.pem",
		"--key=/certs/client-key.pem",
	}
	dArgs = append(dArgs, args...)
	cmd := exec.Command("docker", dArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// writeConfigEtcd writes the flannel network config for the given backend to
// etcd, retrying until it succeeds (etcd may still be coming up).
func writeConfigEtcd(endpoint, certsDir, flannelNetCIDR, backend string) error {
	return writeConfigEtcdKey(endpoint, certsDir, "/coreos.com/network/config",
		fmt.Sprintf(`{ "Network": "%s", "Backend": { "Type": "%s" } }`, flannelNetCIDR, backend))
}

// writeConfigEtcdKey writes an arbitrary key/value to etcd, retrying until it
// succeeds.
func writeConfigEtcdKey(endpoint, certsDir, key, value string) error {
	for {
		_, err := etcdctl(endpoint, certsDir, "put", key, value)
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// waitForFile polls until `docker exec <container> ls <path>` succeeds or the
// container exits with a non-running status.
func waitForFile(container, path string) error {
	for {
		_, err := exec.Command("docker", "exec", container, "ls", path).Output()
		if err == nil {
			return nil
		}
		status := dockerInspectStatus(container)
		if status != "running" {
			return fmt.Errorf("container %s exited with status %q before %s appeared", container, status, path)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// createPingDest adds a dummy interface with the container's FLANNEL_SUBNET and
// returns the bare IP (the first address in the subnet).
func createPingDest(container string) (string, error) {
	out, err := dockerExec(container, true,
		"/bin/sh", "-c",
		`source /run/flannel/subnet.env && \
		ip link add name dummy0 type dummy && \
		ip addr add $FLANNEL_SUBNET dev dummy0 && ip link set dummy0 up && \
		echo $FLANNEL_SUBNET | cut -f 1 -d "/"`,
	)
	if err != nil {
		return "", fmt.Errorf("createPingDest %s: %w\n%s", container, err, out)
	}
	return strings.TrimSpace(out), nil
}

// pings runs bidirectional pings between two flannel containers, binding to
// their respective ping destination IPs.
func pings(container1, pingDest1, container2, pingDest2 string) error {
	if out, err := dockerExec(container1, true, "/bin/ping", "-I", pingDest1, "-c", "3", pingDest2); err != nil {
		return fmt.Errorf("host1 cannot ping host2: %w\n%s", err, out)
	}
	if out, err := dockerExec(container2, true, "/bin/ping", "-I", pingDest2, "-c", "3", pingDest1); err != nil {
		return fmt.Errorf("host2 cannot ping host1: %w\n%s", err, out)
	}
	return nil
}

// logf writes to GinkgoWriter.
func logf(format string, args ...any) {
	_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, format+"\n", args...)
}

// runCommand runs a command and returns its output; used for openssl etc.
func runCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}
