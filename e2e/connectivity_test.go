//go:build e2e

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
package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// backendSpec describes a flannel backend under test.
type backendSpec struct {
	name       string
	backend    string // flannel backend type (name without the _nft suffix)
	enableNFT  bool
	amd64Only  bool
	checkRules func(kc *kindCluster, ctx context.Context)
	hasPerf    bool
}

var backendSpecs = []backendSpec{
	{name: "vxlan", backend: "vxlan", checkRules: func(kc *kindCluster, ctx context.Context) { kc.checkIptables(ctx) }, hasPerf: true},
	{name: "vxlan-nftables", backend: "vxlan", enableNFT: true, checkRules: func(kc *kindCluster, ctx context.Context) { kc.checkNftables(ctx) }},
	{name: "wireguard", backend: "wireguard", checkRules: func(kc *kindCluster, ctx context.Context) { kc.checkIptables(ctx) }, hasPerf: true},
	{name: "host-gw", backend: "host-gw", checkRules: func(kc *kindCluster, ctx context.Context) { kc.checkIptables(ctx) }, hasPerf: true},
	{name: "ipip", backend: "ipip", checkRules: func(kc *kindCluster, ctx context.Context) { kc.checkIptables(ctx) }, hasPerf: true},
	{name: "udp", backend: "udp", amd64Only: true, checkRules: func(kc *kindCluster, ctx context.Context) { kc.checkIptables(ctx) }, hasPerf: true},
}

var _ = Describe("flannel backends", func() {
	for i := range backendSpecs {
		spec := backendSpecs[i]

		Context(spec.name, Ordered, func() {
			var kc *kindCluster

			BeforeAll(func() {
				if spec.amd64Only && arch != "amd64" {
					Skip("backend " + spec.backend + " is only tested on amd64")
				}
				var err error
				kc, err = createCluster()
				Expect(err).NotTo(HaveOccurred(), "creating kind cluster")
			})

			AfterAll(func() {
				if kc != nil {
					Expect(kc.delete()).To(Succeed())
				}
			})

			AfterEach(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					kc.dumpDebugInfo(ctx)
				}
			})

			It("provides pod-to-pod connectivity", func(ctx SpecContext) {
				prepareTest(ctx, kc, spec.backend, spec.enableNFT)
				pings(ctx, kc)
				if spec.checkRules != nil {
					spec.checkRules(kc, ctx)
				}
				Expect(kc.deleteFlannel(ctx)).To(Succeed())
			})

			if spec.hasPerf {
				It("passes iperf3 throughput test", func(ctx SpecContext) {
					prepareTest(ctx, kc, spec.backend, spec.enableNFT)
					perf(ctx, kc)
					Expect(kc.deleteFlannel(ctx)).To(Succeed())
				})
			}
		})
	}
})

// prepareTest installs flannel and waits for the cluster to be ready, mirroring
// the bash prepare_test function.
func prepareTest(ctx context.Context, kc *kindCluster, backend string, enableNFT bool) {
	GinkgoHelper()

	By("installing flannel with backend " + backend)
	Expect(kc.installFlannel(ctx, backend, enableNFT)).To(Succeed())

	By("waiting for the flannel DaemonSet to roll out")
	Expect(kc.waitForFlannelRollout(ctx, 5*time.Minute)).To(Succeed())

	By("waiting for /run/flannel/subnet.env on all nodes")
	Expect(kc.waitForSubnetEnv(ctx, 2*time.Minute)).To(Succeed())

	By("waiting for nodes to be ready")
	Expect(kc.waitForNodesReady(ctx, 5*time.Minute)).To(Succeed())

	By("waiting for coredns to be ready")
	Expect(kc.waitForCoreDNS(ctx, 5*time.Minute)).To(Succeed())
}

// pings creates two multitool pods on separate nodes and verifies bidirectional
// connectivity, mirroring the bash pings function.
func pings(ctx context.Context, kc *kindCluster) {
	GinkgoHelper()

	Expect(kc.createTestPod(ctx, "multitool1", kindWorker)).To(Succeed())
	Expect(kc.createTestPod(ctx, "multitool2", kindControlPlane)).To(Succeed())

	By("waiting for test pods to be ready")
	Expect(kc.waitForPodReady(ctx, "default", "multitool1", time.Minute)).To(Succeed())
	Expect(kc.waitForPodReady(ctx, "default", "multitool2", time.Minute)).To(Succeed())

	ip1, err := kc.podIP(ctx, "multitool1")
	Expect(err).NotTo(HaveOccurred())
	ip2, err := kc.podIP(ctx, "multitool2")
	Expect(err).NotTo(HaveOccurred())
	By("multitool1=" + ip1 + " multitool2=" + ip2)

	Expect(kc.waitForPing(ctx, "multitool1", ip2, time.Minute)).To(Succeed())

	_, err = kc.execInPod(ctx, "default", "multitool1", "ping", "-c", "5", ip2)
	Expect(err).NotTo(HaveOccurred(), "multitool1 cannot ping multitool2")
	_, err = kc.execInPod(ctx, "default", "multitool2", "ping", "-c", "5", ip1)
	Expect(err).NotTo(HaveOccurred(), "multitool2 cannot ping multitool1")
}

// perf runs an iperf3 throughput test between two pods, mirroring the bash perf
// function.
func perf(ctx context.Context, kc *kindCluster) {
	GinkgoHelper()

	Expect(kc.createIperfServerPod(ctx, "iperf3-server", kindWorker)).To(Succeed())
	Expect(kc.createIperfClientPod(ctx, "iperf3-client", kindControlPlane)).To(Succeed())

	By("waiting for iperf3 pods to be ready")
	Expect(kc.waitForPodReady(ctx, "default", "iperf3-server", time.Minute)).To(Succeed())
	Expect(kc.waitForPodReady(ctx, "default", "iperf3-client", time.Minute)).To(Succeed())

	serverIP, err := kc.podIP(ctx, "iperf3-server")
	Expect(err).NotTo(HaveOccurred())

	// Give the server a moment to start listening.
	time.Sleep(5 * time.Second)

	By("running iperf3 client against " + serverIP)
	out, err := kc.execInPod(ctx, "default", "iperf3-client", "iperf3", "-c", serverIP, "-t", "10")
	Expect(err).NotTo(HaveOccurred(), "iperf3 client failed: %s", out)
}
