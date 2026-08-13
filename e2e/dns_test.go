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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
)

// The service/DNS spec exercises a ClusterIP Service backed by a multi-replica
// Deployment plus CoreDNS resolution over the flannel network. It uses a single
// cheap backend (vxlan, iptables) since its purpose is to validate service
// routing and cluster DNS rather than the full backend matrix. It assumes
// CoreDNS and kube-proxy are healthy in the shared cluster.
var _ = Describe("flannel service and DNS", Ordered, func() {
	const (
		deploymentName = "dns-web"
		serviceName    = "dns-web"
		clientPod      = "dns-client"
		servicePort    = int32(80)
		fqdn           = "dns-web.default.svc.cluster.local"
	)

	BeforeAll(func() {
		if enableIPv6 {
			Skip("service/DNS spec runs on the IPv4-only matrix (skipped when IP_FAMILY=dual)")
		}
	})

	AfterEach(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() {
			sharedCluster.dumpDebugInfo(ctx)
		}
	})

	AfterAll(func(ctx SpecContext) {
		By("cleaning up service/DNS test resources")
		Expect(sharedCluster.deletePod(ctx, "default", clientPod)).To(Succeed())
		Expect(sharedCluster.deleteService(ctx, "default", serviceName)).To(Succeed())
		Expect(sharedCluster.deleteDeployment(ctx, "default", deploymentName)).To(Succeed())
		Expect(sharedCluster.deleteFlannel(ctx)).To(Succeed())
	})

	It("resolves and reaches a ClusterIP service across nodes", func(ctx SpecContext) {
		prepareTest(ctx, sharedCluster, "vxlan", false, false)

		By("creating the backend deployment spread across nodes")
		Expect(sharedCluster.createDeployment(ctx, deploymentName, multitoolImage, 2, servicePort, "")).To(Succeed())
		Expect(sharedCluster.waitForDeploymentReady(ctx, "default", deploymentName, 2*time.Minute)).To(Succeed())

		By("creating the ClusterIP service")
		Expect(sharedCluster.createClusterIPService(ctx, serviceName,
			map[string]string{"app": deploymentName}, servicePort, servicePort)).To(Succeed())
		clusterIP, err := sharedCluster.serviceClusterIP(ctx, "default", serviceName)
		Expect(err).NotTo(HaveOccurred())
		Expect(clusterIP).NotTo(BeEmpty())
		By("service " + serviceName + " ClusterIP=" + clusterIP)

		By("creating the client pod on the control-plane node")
		Expect(sharedCluster.createTestPod(ctx, clientPod, kindControlPlane)).To(Succeed())
		Expect(sharedCluster.waitForPodReady(ctx, "default", clientPod, time.Minute)).To(Succeed())

		By("resolving kubernetes.default via CoreDNS")
		Expect(sharedCluster.waitForDNS(ctx, clientPod, "kubernetes.default", time.Minute)).
			To(Succeed(), "CoreDNS did not resolve kubernetes.default over the flannel network")

		By("resolving the service FQDN to its ClusterIP")
		out, err := sharedCluster.execInPod(ctx, "default", clientPod, "nslookup", fqdn)
		Expect(err).NotTo(HaveOccurred(), "nslookup %s failed: %s", fqdn, out)
		Expect(out).To(ContainSubstring(clusterIP),
			"nslookup did not return the service ClusterIP %s:\n%s", clusterIP, out)

		By("curling the service by DNS name")
		Expect(sharedCluster.waitForHTTPOK(ctx, clientPod, "http://"+fqdn, time.Minute)).
			To(Succeed(), "service was not reachable by DNS name over the flannel network")

		By("curling the service by raw ClusterIP")
		Expect(sharedCluster.waitForHTTPOK(ctx, clientPod, "http://"+clusterIP, time.Minute)).
			To(Succeed(), "service was not reachable by ClusterIP over the flannel network")
	})
})

// waitForDNS polls until nslookup of host from pod returns an answer section,
// mirroring the style of waitForPing.
func (kc *kindCluster) waitForDNS(ctx context.Context, pod, host string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		out, err := kc.execInPod(ctx, "default", pod, "nslookup", host)
		if err != nil {
			return false, nil
		}
		lowerOut := strings.ToLower(out)
		for _, failure := range []string{"can't resolve", "can't find", "nxdomain", "no answer", "server can't"} {
			if strings.Contains(lowerOut, failure) {
				return false, nil
			}
		}

		seenName := false
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Name:") {
				seenName = true
				continue
			}
			if seenName && line != "" && strings.Contains(line, "Address") {
				return true, nil
			}
		}
		return false, nil
	})
}

// waitForHTTPOK polls until an HTTP GET of url from pod returns 200, mirroring
// the style of waitForPing. kube-proxy and pod readiness race, so this retries.
func (kc *kindCluster) waitForHTTPOK(ctx context.Context, pod, url string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		out, err := kc.execInPod(ctx, "default", pod,
			"curl", "-sS", "-o", "/dev/null", "-w", "%{http_code}", "--max-time", "5", url)
		if err != nil {
			return false, nil
		}
		return strings.TrimSpace(out) == "200", nil
	})
}
