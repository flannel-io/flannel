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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The resilience suite exercises recovery paths on a single cheap backend
// (vxlan, iptables) against the shared kind cluster. It deliberately avoids full
// node reboots / `docker restart` of a kind node, which are too flaky and slow
// for CI; the pod restart and DaemonSet rolling restart below cover the
// realistic flanneld-recovery paths (pod re-wiring and lease persistence).
var _ = Describe("flannel resilience", Ordered, func() {
	BeforeAll(func(ctx SpecContext) {
		if enableIPv6 {
			Skip("resilience suite runs on the IPv4-only matrix")
		}
		// Install flannel with the vxlan backend (iptables) and wait for the
		// cluster to be fully ready, reusing the shared prepareTest helper.
		prepareTest(ctx, sharedCluster, "vxlan", false, false)
	})

	AfterEach(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() {
			sharedCluster.dumpDebugInfo(ctx)
		}
	})

	AfterAll(func(ctx SpecContext) {
		// Leave the shared cluster clean for any later specs.
		Expect(sharedCluster.deleteFlannel(ctx)).To(Succeed())
	})

	It("recovers pod connectivity after a pod is deleted and recreated", func(ctx SpecContext) {
		By("creating two multitool pods on separate nodes")
		Expect(sharedCluster.createTestPod(ctx, "multitool1", kindWorker)).To(Succeed())
		Expect(sharedCluster.createTestPod(ctx, "multitool2", kindControlPlane)).To(Succeed())
		Expect(sharedCluster.waitForPodReady(ctx, "default", "multitool1", time.Minute)).To(Succeed())
		Expect(sharedCluster.waitForPodReady(ctx, "default", "multitool2", time.Minute)).To(Succeed())

		By("verifying initial cross-node connectivity")
		ip2, err := sharedCluster.podIP(ctx, "multitool2")
		Expect(err).NotTo(HaveOccurred())
		Expect(sharedCluster.waitForPing(ctx, "multitool1", ip2, time.Minute)).To(Succeed())

		By("deleting multitool1 and recreating it on the same node")
		Expect(sharedCluster.deletePod(ctx, "default", "multitool1")).To(Succeed())
		Expect(sharedCluster.createTestPod(ctx, "multitool1", kindWorker)).To(Succeed())
		Expect(sharedCluster.waitForPodReady(ctx, "default", "multitool1", time.Minute)).To(Succeed())

		// The recreated pod gets a fresh IP within the node's subnet, proving
		// flannel re-wired a new pod/veth and re-allocated an address.
		By("verifying connectivity recovers for the recreated pod")
		newIP1, err := sharedCluster.podIP(ctx, "multitool1")
		Expect(err).NotTo(HaveOccurred())
		By("recreated multitool1=" + newIP1 + " multitool2=" + ip2)

		Expect(sharedCluster.waitForPing(ctx, "multitool1", ip2, time.Minute)).To(Succeed())
		_, err = sharedCluster.execInPod(ctx, "default", "multitool1", "ping", "-c", "5", ip2)
		Expect(err).NotTo(HaveOccurred(), "recreated multitool1 cannot ping multitool2")
		_, err = sharedCluster.execInPod(ctx, "default", "multitool2", "ping", "-c", "5", newIP1)
		Expect(err).NotTo(HaveOccurred(), "multitool2 cannot ping recreated multitool1")
	})

	It("preserves connectivity and subnet leases across a flannel DaemonSet restart", func(ctx SpecContext) {
		By("recreating both test pods on separate nodes")
		Expect(sharedCluster.deletePod(ctx, "default", "multitool1")).To(Succeed())
		Expect(sharedCluster.deletePod(ctx, "default", "multitool2")).To(Succeed())
		Expect(sharedCluster.createTestPod(ctx, "multitool1", kindWorker)).To(Succeed())
		Expect(sharedCluster.createTestPod(ctx, "multitool2", kindControlPlane)).To(Succeed())
		Expect(sharedCluster.waitForPodReady(ctx, "default", "multitool1", time.Minute)).To(Succeed())
		Expect(sharedCluster.waitForPodReady(ctx, "default", "multitool2", time.Minute)).To(Succeed())

		ip1, err := sharedCluster.podIP(ctx, "multitool1")
		Expect(err).NotTo(HaveOccurred())
		ip2, err := sharedCluster.podIP(ctx, "multitool2")
		Expect(err).NotTo(HaveOccurred())
		Expect(sharedCluster.waitForPing(ctx, "multitool1", ip2, time.Minute)).To(Succeed())

		// Capture each node's FLANNEL_SUBNET lease BEFORE the restart so we can
		// assert it is unchanged afterwards (leases must survive a flanneld
		// restart).
		By("capturing subnet.env leases before the restart")
		nodes := []string{kindControlPlane, kindWorker}
		subnetBefore := make(map[string]string, len(nodes))
		for _, node := range nodes {
			env, err := sharedCluster.subnetEnv(ctx, node)
			Expect(err).NotTo(HaveOccurred(), "reading subnet.env on %s", node)
			line := flannelSubnetLine(env)
			Expect(line).NotTo(BeEmpty(), "no FLANNEL_SUBNET on %s:\n%s", node, env)
			subnetBefore[node] = line
		}

		By("triggering a rolling restart of the flannel DaemonSet")
		Expect(sharedCluster.restartFlannelDaemonSet(ctx)).To(Succeed())
		Expect(sharedCluster.waitForFlannelRollout(ctx, 5*time.Minute)).To(Succeed())
		Expect(sharedCluster.waitForSubnetEnv(ctx, 2*time.Minute)).To(Succeed())

		// Leases (and thus the FLANNEL_SUBNET line) must be stable across the
		// restart, proving subnet lease persistence.
		By("verifying subnet.env leases are unchanged after the restart")
		for _, node := range nodes {
			env, err := sharedCluster.subnetEnv(ctx, node)
			Expect(err).NotTo(HaveOccurred(), "reading subnet.env on %s after restart", node)
			Expect(flannelSubnetLine(env)).To(Equal(subnetBefore[node]),
				"FLANNEL_SUBNET changed on %s across restart", node)
		}

		By("verifying pod-to-pod connectivity still works after the restart")
		Expect(sharedCluster.waitForPing(ctx, "multitool1", ip2, time.Minute)).To(Succeed())
		_, err = sharedCluster.execInPod(ctx, "default", "multitool1", "ping", "-c", "5", ip2)
		Expect(err).NotTo(HaveOccurred(), "multitool1 cannot ping multitool2 after restart")
		_, err = sharedCluster.execInPod(ctx, "default", "multitool2", "ping", "-c", "5", ip1)
		Expect(err).NotTo(HaveOccurred(), "multitool2 cannot ping multitool1 after restart")
	})
})
