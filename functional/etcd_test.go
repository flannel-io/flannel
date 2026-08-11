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
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// certsDir is the path to the TLS certs used by the etcd container.
// Equivalent to `${PWD}/test` in the bash suite, but the bash suite ran from
// dist/ so it resolves to dist/test.
var certsDir = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "dist", "test")
}()

// etcdBackend describes a flannel backend to test against a raw etcd.
type etcdBackend struct {
	name      string
	amd64Only bool
	hasPerf   bool
}

var etcdBackends = []etcdBackend{
	{name: "vxlan"},
	{name: "udp", amd64Only: true},
	{name: "host-gw"},
	{name: "ipip"},
	{name: "ipsec", hasPerf: true},
	{name: "wireguard"},
}

var _ = Describe("etcd backends", Ordered, func() {
	var etcdEndpt string

	BeforeAll(func() {
		dockerIP, err := docker0IP()
		Expect(err).NotTo(HaveOccurred(), "getting docker0 IP")
		etcdEndpt = "http://" + dockerIP + ":2379"

		dockerRm("flannel-e2e-test-etcd")
		absDir, err := filepath.Abs(certsDir)
		Expect(err).NotTo(HaveOccurred())
		_, err = dockerRun(
			"--name=flannel-e2e-test-etcd", "-d", "--dns", "8.8.8.8",
			"-v", absDir+":/certs",
			"-e", "ETCD_UNSUPPORTED_ARCH="+arch,
			"-p", "2379:2379",
			etcdImg, etcdLocation,
			"--listen-client-urls", "http://0.0.0.0:2379",
			"--cert-file=/certs/server.pem",
			"--key-file=/certs/server-key.pem",
			"--client-cert-auth",
			"--trusted-ca-file=/certs/ca.pem",
			"--advertise-client-urls", etcdEndpt,
		)
		Expect(err).NotTo(HaveOccurred(), "starting etcd")
	})

	AfterAll(func() {
		dockerRm("flannel-e2e-test-etcd")
	})

	AfterEach(func() {
		absDir, _ := filepath.Abs(certsDir)

		// Always dump state for debugging.
		By("dumping subnets in etcd")
		out, _ := etcdctl(etcdEndpt, absDir, "get", "--prefix", "/coreos.com/network/subnets")
		logf("etcd subnets:\n%s", out)

		logf("### logs for flannel-e2e-test-flannel1 ###\n%s", dockerLogs("flannel-e2e-test-flannel1"))
		logf("### logs for flannel-e2e-test-flannel2 ###\n%s", dockerLogs("flannel-e2e-test-flannel2"))

		dockerRm("flannel-e2e-test-flannel1")
		dockerRm("flannel-e2e-test-flannel2")
		dockerRm("flannel-e2e-test-flannel1-iperf")
		dockerRm("flannel-host1")
		dockerRm("flannel-host2")

		// Clean config key so the next test starts fresh.
		etcdctl(etcdEndpt, absDir, "del", "/coreos.com/network/config") //nolint:errcheck
	})

	// startFlannelContainers starts two privileged flannel containers connected
	// to the TLS etcd.
	startFlannelContainers := func(etcdEndpt string) {
		GinkgoHelper()
		absDir, err := filepath.Abs(certsDir)
		Expect(err).NotTo(HaveOccurred())

		for _, num := range []string{"1", "2"} {
			name := "flannel-e2e-test-flannel" + num
			dockerRm(name)
			_, err := dockerRun(
				"-v", absDir+":/certs",
				"--name="+name, "-d", "--privileged",
				flannelDockerImage,
				"--etcd-cafile=/certs/ca.pem",
				"--etcd-certfile=/certs/client.pem",
				"--etcd-keyfile=/certs/client-key.pem",
				"--etcd-endpoints="+etcdEndpt,
				"-v", "10",
			)
			Expect(err).NotTo(HaveOccurred(), "starting flannel container %s", name)
		}
	}

	for i := range etcdBackends {
		spec := etcdBackends[i]

		Context(spec.name, func() {
			BeforeEach(func() {
				if spec.amd64Only && arch != "amd64" {
					Skip("backend " + spec.name + " is only tested on amd64")
				}
			})

			It("pings between two flannel hosts", func() {
				absDir, err := filepath.Abs(certsDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(writeConfigEtcd(etcdEndpt, absDir, flannelNet, spec.name)).To(Succeed())
				startFlannelContainers(etcdEndpt)

				By("waiting for subnet.env on both containers")
				Expect(waitForFile("flannel-e2e-test-flannel1", "/run/flannel/subnet.env")).To(Succeed())
				Expect(waitForFile("flannel-e2e-test-flannel2", "/run/flannel/subnet.env")).To(Succeed())

				pingDest1, err := createPingDest("flannel-e2e-test-flannel1")
				Expect(err).NotTo(HaveOccurred())
				pingDest2, err := createPingDest("flannel-e2e-test-flannel2")
				Expect(err).NotTo(HaveOccurred())
				By(fmt.Sprintf("pingDest1=%s pingDest2=%s", pingDest1, pingDest2))

				Expect(pings("flannel-e2e-test-flannel1", pingDest1, "flannel-e2e-test-flannel2", pingDest2)).
					To(Succeed())
			})

			if spec.hasPerf {
				It("passes iperf3 throughput test", func() {
					absDir, err := filepath.Abs(certsDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(writeConfigEtcd(etcdEndpt, absDir, flannelNet, spec.name)).To(Succeed())
					startFlannelContainers(etcdEndpt)

					By("waiting for subnet.env on both containers")
					Expect(waitForFile("flannel-e2e-test-flannel1", "/run/flannel/subnet.env")).To(Succeed())
					Expect(waitForFile("flannel-e2e-test-flannel2", "/run/flannel/subnet.env")).To(Succeed())

					pingDest1, err := createPingDest("flannel-e2e-test-flannel1")
					Expect(err).NotTo(HaveOccurred())
					pingDest2, err := createPingDest("flannel-e2e-test-flannel2")
					Expect(err).NotTo(HaveOccurred())

					dockerRm("flannel-e2e-test-flannel1-iperf")
					_, err = dockerRun(
						"-d", "--name=flannel-e2e-test-flannel1-iperf",
						"--net=container:flannel-e2e-test-flannel1",
						"iperf3:latest",
					)
					Expect(err).NotTo(HaveOccurred(), "starting iperf3 server")

					// Wait for iperf3 server container to be running.
					Eventually(func() string {
						return dockerInspectStatus("flannel-e2e-test-flannel1-iperf")
					}, 30*time.Second, time.Second).Should(Equal("running"))

					out, err := runCommand(
						"docker", "run", "--rm",
						"--net=container:flannel-e2e-test-flannel2",
						"iperf3:latest",
						"-c", pingDest1, "-B", pingDest2,
					)
					Expect(err).NotTo(HaveOccurred(), "iperf3 client failed: %s", out)
				})
			}
		})
	}

	Context("multi (vxlan+host-gw dual networks)", func() {
		It("routes traffic over the correct backend", func() {
			absDir, err := filepath.Abs(certsDir)
			Expect(err).NotTo(HaveOccurred())
			dockerIP, err := docker0IP()
			Expect(err).NotTo(HaveOccurred())
			etcdEndpt := "http://" + dockerIP + ":2379"

			// Write configs for both networks.
			Expect(writeConfigEtcdKey(etcdEndpt, absDir, "/vxlan/network/config",
				`{"Network": "10.11.0.0/16", "Backend": {"Type": "vxlan"}}`)).To(Succeed())
			Expect(writeConfigEtcdKey(etcdEndpt, absDir, "/hostgw/network/config",
				`{"Network": "10.12.0.0/16", "Backend": {"Type": "host-gw"}}`)).To(Succeed())

			for _, num := range []string{"1", "2"} {
				name := "flannel-host" + num
				dockerRm(name)
				_, err := dockerRun(
					"-v", absDir+":/certs",
					"--name="+name, "-id", "--privileged",
					"--entrypoint", "/bin/sh",
					flannelDockerImage,
				)
				Expect(err).NotTo(HaveOccurred(), "starting host container %s", name)

				// Start two flanneld instances inside the container.
				_, err = dockerExec(name, false, "sh", "-c",
					fmt.Sprintf(`/opt/bin/flanneld -v 10 -subnet-file /vxlan.env -etcd-prefix=/vxlan/network --etcd-cafile=/certs/ca.pem --etcd-certfile=/certs/client.pem --etcd-keyfile=/certs/client-key.pem --etcd-endpoints=%s 2>vxlan.log &`, etcdEndpt))
				Expect(err).NotTo(HaveOccurred())
				_, err = dockerExec(name, false, "sh", "-c",
					fmt.Sprintf(`/opt/bin/flanneld -v 10 -subnet-file /hostgw.env -etcd-prefix=/hostgw/network --etcd-cafile=/certs/ca.pem --etcd-certfile=/certs/client.pem --etcd-keyfile=/certs/client-key.pem --etcd-endpoints=%s 2>hostgw.log &`, etcdEndpt))
				Expect(err).NotTo(HaveOccurred())
			}

			// Wait for both env files on both hosts.
			for _, num := range []string{"1", "2"} {
				host := "flannel-host" + num
				Expect(waitForFile(host, "/vxlan.env")).To(Succeed())
				Expect(waitForFile(host, "/hostgw.env")).To(Succeed())
			}

			// Create dummy interfaces on host1 and capture IPs.
			vxlanPingDest, err := dockerExec("flannel-host1", false, "/bin/sh", "-c",
				`source /vxlan.env && \
				ip link add name dummy_vxlan type dummy && \
				ip addr add $FLANNEL_SUBNET dev dummy_vxlan && \
				ip link set dummy_vxlan up && \
				echo $FLANNEL_SUBNET | cut -f 1 -d "/"`)
			Expect(err).NotTo(HaveOccurred())
			vxlanPingDest = trimOutput(vxlanPingDest)

			hostgwPingDest, err := dockerExec("flannel-host1", false, "/bin/sh", "-c",
				`source /hostgw.env && \
				ip link add name dummy_hostgw type dummy && \
				ip addr add $FLANNEL_SUBNET dev dummy_hostgw && \
				ip link set dummy_hostgw up && \
				echo $FLANNEL_SUBNET | cut -f 1 -d "/"`)
			Expect(err).NotTo(HaveOccurred())
			hostgwPingDest = trimOutput(hostgwPingDest)

			By(fmt.Sprintf("vxlanPingDest=%s hostgwPingDest=%s", vxlanPingDest, hostgwPingDest))

			// Correct-interface pings should succeed.
			_, err = dockerExec("flannel-host2", false, "ping", "-c", "3", hostgwPingDest)
			Expect(err).NotTo(HaveOccurred(), "host2 ping over host-gw should succeed")
			_, err = dockerExec("flannel-host2", false, "ping", "-c", "3", vxlanPingDest)
			Expect(err).NotTo(HaveOccurred(), "host2 ping over vxlan should succeed")

			// Wrong-interface pings should fail.
			_, err = dockerExec("flannel-host2", false, "ping", "-W", "1", "-c", "1", "-I", "flannel.1", hostgwPingDest)
			Expect(err).To(HaveOccurred(), "ping over flannel.1 to host-gw dest should fail")
			_, err = dockerExec("flannel-host2", false, "ping", "-W", "1", "-c", "1", "-I", "eth0", vxlanPingDest)
			Expect(err).To(HaveOccurred(), "ping over eth0 to vxlan dest should fail")

			// Clean up multi-network keys so AfterEach doesn't error on the
			// normal /coreos.com/network/config key.
			etcdctl(etcdEndpt, absDir, "del", "/vxlan/network/config")   //nolint:errcheck
			etcdctl(etcdEndpt, absDir, "del", "/hostgw/network/config")  //nolint:errcheck
		})
	})
})

// trimOutput trims whitespace and trailing newlines from command output.
func trimOutput(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[len(lines)-1])
}
