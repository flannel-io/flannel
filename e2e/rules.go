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
	"fmt"
	"io"
	"strings"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

// expectedIptablesPostrouting builds the golden FLANNEL-POSTRTG rule set for a
// node with the given pod CIDR, mirroring check_iptables.
func expectedIptablesPostrouting(podCIDR string) string {
	return iptablesPostroutingRules(podCIDR, flannelNet, "224.0.0.0/4")
}

// expectedIp6tablesPostrouting is the IPv6 counterpart (ff00::/8 multicast).
func expectedIp6tablesPostrouting(podCIDR string) string {
	return iptablesPostroutingRules(podCIDR, flannelIPv6Net, "ff00::/8")
}

func iptablesPostroutingRules(podCIDR, network, multicast string) string {
	return strings.Join([]string{
		`-A POSTROUTING -m comment --comment "flanneld masq" -j FLANNEL-POSTRTG`,
		`-N FLANNEL-POSTRTG`,
		`-A FLANNEL-POSTRTG -m mark --mark 0x4000/0x4000 -m comment --comment "flanneld masq" -j RETURN`,
		fmt.Sprintf(`-A FLANNEL-POSTRTG -s %s -d %s -m comment --comment "flanneld masq" -j RETURN`, podCIDR, network),
		fmt.Sprintf(`-A FLANNEL-POSTRTG -s %s -d %s -m comment --comment "flanneld masq" -j RETURN`, network, podCIDR),
		fmt.Sprintf(`-A FLANNEL-POSTRTG ! -s %s -d %s -m comment --comment "flanneld masq" -j RETURN`, network, podCIDR),
		fmt.Sprintf(`-A FLANNEL-POSTRTG -s %s ! -d %s -m comment --comment "flanneld masq" -j MASQUERADE --random-fully`, network, multicast),
		fmt.Sprintf(`-A FLANNEL-POSTRTG ! -s %s -d %s -m comment --comment "flanneld masq" -j MASQUERADE --random-fully`, network, network),
	}, "\n")
}

func expectedIptablesForward() string {
	return iptablesForwardRules(flannelNet)
}

// expectedIp6tablesForward is the IPv6 counterpart of expectedIptablesForward.
func expectedIp6tablesForward() string {
	return iptablesForwardRules(flannelIPv6Net)
}

func iptablesForwardRules(network string) string {
	return strings.Join([]string{
		`-P FORWARD ACCEPT`,
		`-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes load balancer firewall" -j KUBE-PROXY-FIREWALL`,
		`-A FORWARD -m comment --comment "kubernetes forwarding rules" -j KUBE-FORWARD`,
		`-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes service portals" -j KUBE-SERVICES`,
		`-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes externally-visible service portals" -j KUBE-EXTERNAL-SERVICES`,
		`-A FORWARD -m comment --comment "flanneld forward" -j FLANNEL-FWD`,
		`-N FLANNEL-FWD`,
		fmt.Sprintf(`-A FLANNEL-FWD -s %s -m comment --comment "flanneld forward" -j ACCEPT`, network),
		fmt.Sprintf(`-A FLANNEL-FWD -d %s -m comment --comment "flanneld forward" -j ACCEPT`, network),
	}, "\n")
}

// checkRules asserts flannel's masquerade and forward rules on both nodes, using
// nftables when enableNFT is set and iptables otherwise.
func (kc *kindCluster) checkRules(ctx context.Context, enableNFT bool) {
	if enableNFT {
		kc.checkNftables(ctx)
	} else {
		kc.checkIptables(ctx)
	}
}

// checkRulesV6 asserts flannel's IPv6 masquerade and forward rules on both nodes.
func (kc *kindCluster) checkRulesV6(ctx context.Context, enableNFT bool) {
	if enableNFT {
		kc.checkNftablesV6(ctx)
	} else {
		kc.checkIp6tables(ctx)
	}
}

// nodeCIDRs returns the pod CIDR of each node keyed by node name.
func (kc *kindCluster) nodeCIDRs(ctx context.Context) map[string]string {
	cidrs := map[string]string{}
	for _, node := range []string{kindWorker, kindControlPlane} {
		cidr, err := kc.podCIDR(ctx, node)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		cidrs[node] = cidr
	}
	return cidrs
}

// nodeCIDRsV6 returns the IPv6 pod CIDR of each node keyed by node name.
func (kc *kindCluster) nodeCIDRsV6(ctx context.Context) map[string]string {
	cidrs := map[string]string{}
	for _, node := range []string{kindWorker, kindControlPlane} {
		cidr, err := kc.podCIDRv6(ctx, node)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		cidrs[node] = cidr
	}
	return cidrs
}

// checkIptables asserts the masquerade and forward rules on both nodes, mirroring
// the bash check_iptables helper.
func (kc *kindCluster) checkIptables(ctx context.Context) {
	for node, cidr := range kc.nodeCIDRs(ctx) {
		gomega.Expect(kc.iptablesNatFlannel(node, "/usr/sbin/iptables")).To(gomega.Equal(expectedIptablesPostrouting(cidr)),
			"node %s has unexpected postrouting rules", node)
		gomega.Expect(kc.iptablesFilterForward(node, "/usr/sbin/iptables")).To(gomega.Equal(expectedIptablesForward()),
			"node %s has unexpected forward rules", node)
	}
}

// checkIp6tables asserts the IPv6 masquerade and forward rules on both nodes.
func (kc *kindCluster) checkIp6tables(ctx context.Context) {
	for node, cidr := range kc.nodeCIDRsV6(ctx) {
		gomega.Expect(kc.iptablesNatFlannel(node, "/usr/sbin/ip6tables")).To(gomega.Equal(expectedIp6tablesPostrouting(cidr)),
			"node %s has unexpected IPv6 postrouting rules", node)
		gomega.Expect(kc.iptablesFilterForward(node, "/usr/sbin/ip6tables")).To(gomega.Equal(expectedIp6tablesForward()),
			"node %s has unexpected IPv6 forward rules", node)
	}
}

func (kc *kindCluster) iptablesNatFlannel(node, bin string) string {
	post, err := kc.execOnNode(node, bin, "-t", "nat", "-S", "POSTROUTING")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	chain, err := kc.execOnNode(node, bin, "-t", "nat", "-S", "FLANNEL-POSTRTG")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	var lines []string
	for _, l := range strings.Split(strings.TrimRight(post, "\n"), "\n") {
		if strings.Contains(l, "FLANNEL") {
			lines = append(lines, l)
		}
	}
	lines = append(lines, strings.Split(strings.TrimRight(chain, "\n"), "\n")...)
	return strings.Join(lines, "\n")
}

func (kc *kindCluster) iptablesFilterForward(node, bin string) string {
	fwd, err := kc.execOnNode(node, bin, "-t", "filter", "-S", "FORWARD")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	chain, err := kc.execOnNode(node, bin, "-t", "filter", "-S", "FLANNEL-FWD")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return strings.TrimRight(fwd, "\n") + "\n" + strings.TrimRight(chain, "\n")
}

// expectedNftPostrouting builds the golden nftables postrtg chain, mirroring
// check_nftables.
func expectedNftPostrouting(podCIDR string) string {
	return strings.Join([]string{
		`table ip flannel-ipv4 {`,
		"\tchain postrtg {",
		"\t\tcomment \"chain to manage traffic masquerading by flannel\"",
		"\t\ttype nat hook postrouting priority srcnat; policy accept;",
		"\t\tmeta mark 0x00004000 return",
		fmt.Sprintf("\t\tip saddr %s ip daddr %s return", podCIDR, flannelNet),
		fmt.Sprintf("\t\tip saddr %s ip daddr %s return", flannelNet, podCIDR),
		fmt.Sprintf("\t\tip saddr != %s ip daddr %s return", podCIDR, flannelNet),
		fmt.Sprintf("\t\tip saddr %s ip daddr != 224.0.0.0/4 masquerade fully-random", flannelNet),
		fmt.Sprintf("\t\tip saddr != %s ip daddr %s masquerade fully-random", flannelNet, flannelNet),
		"\t}",
		`}`,
	}, "\n")
}

func expectedNftForward() string {
	return strings.Join([]string{
		`table ip flannel-ipv4 {`,
		"\tchain forward {",
		"\t\tcomment \"chain to accept flannel traffic\"",
		"\t\ttype filter hook forward priority filter; policy accept;",
		fmt.Sprintf("\t\tip saddr %s accept", flannelNet),
		fmt.Sprintf("\t\tip daddr %s accept", flannelNet),
		"\t}",
		`}`,
	}, "\n")
}

// checkNftables asserts the nftables masquerade and forward chains on both nodes.
func (kc *kindCluster) checkNftables(ctx context.Context) {
	for node, cidr := range kc.nodeCIDRs(ctx) {
		post, err := kc.execOnNode(node, "/usr/sbin/nft", "list", "chain", "ip", "flannel-ipv4", "postrtg")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(strings.TrimRight(post, "\n")).To(gomega.Equal(expectedNftPostrouting(cidr)),
			"node %s has unexpected nftables postrouting rules", node)

		fwd, err := kc.execOnNode(node, "/usr/sbin/nft", "list", "chain", "ip", "flannel-ipv4", "forward")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(strings.TrimRight(fwd, "\n")).To(gomega.Equal(expectedNftForward()),
			"node %s has unexpected nftables forward rules", node)
	}
}

// expectedNft6Postrouting builds the golden nftables postrtg chain for the IPv6
// family (table ip6 flannel-ipv6), mirroring addMasqRules with ff00::/8.
func expectedNft6Postrouting(podCIDR string) string {
	return strings.Join([]string{
		`table ip6 flannel-ipv6 {`,
		"\tchain postrtg {",
		"\t\tcomment \"chain to manage traffic masquerading by flannel\"",
		"\t\ttype nat hook postrouting priority srcnat; policy accept;",
		"\t\tmeta mark 0x00004000 return",
		fmt.Sprintf("\t\tip6 saddr %s ip6 daddr %s return", podCIDR, flannelIPv6Net),
		fmt.Sprintf("\t\tip6 saddr %s ip6 daddr %s return", flannelIPv6Net, podCIDR),
		fmt.Sprintf("\t\tip6 saddr != %s ip6 daddr %s return", podCIDR, flannelIPv6Net),
		fmt.Sprintf("\t\tip6 saddr %s ip6 daddr != ff00::/8 masquerade fully-random", flannelIPv6Net),
		fmt.Sprintf("\t\tip6 saddr != %s ip6 daddr %s masquerade fully-random", flannelIPv6Net, flannelIPv6Net),
		"\t}",
		`}`,
	}, "\n")
}

func expectedNft6Forward() string {
	return strings.Join([]string{
		`table ip6 flannel-ipv6 {`,
		"\tchain forward {",
		"\t\tcomment \"chain to accept flannel traffic\"",
		"\t\ttype filter hook forward priority filter; policy accept;",
		fmt.Sprintf("\t\tip6 saddr %s accept", flannelIPv6Net),
		fmt.Sprintf("\t\tip6 daddr %s accept", flannelIPv6Net),
		"\t}",
		`}`,
	}, "\n")
}

// checkNftablesV6 asserts the IPv6 nftables masquerade and forward chains.
func (kc *kindCluster) checkNftablesV6(ctx context.Context) {
	for node, cidr := range kc.nodeCIDRsV6(ctx) {
		post, err := kc.execOnNode(node, "/usr/sbin/nft", "list", "chain", "ip6", "flannel-ipv6", "postrtg")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(strings.TrimRight(post, "\n")).To(gomega.Equal(expectedNft6Postrouting(cidr)),
			"node %s has unexpected IPv6 nftables postrouting rules", node)

		fwd, err := kc.execOnNode(node, "/usr/sbin/nft", "list", "chain", "ip6", "flannel-ipv6", "forward")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(strings.TrimRight(fwd, "\n")).To(gomega.Equal(expectedNft6Forward()),
			"node %s has unexpected IPv6 nftables forward rules", node)
	}
}

// dumpDebugInfo prints cluster state for troubleshooting, mirroring
// dump_debug_info from the bash suite. Invoked from AfterEach on failure.
func (kc *kindCluster) dumpDebugInfo(ctx context.Context) {
	if kc == nil || kc.client == nil {
		return
	}
	w := ginkgo.GinkgoWriter
	writeln(w, "======== DEBUG INFO ========")

	writeln(w, "--- nodes ---")
	nodes, err := kc.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, n := range nodes.Items {
			writef(w, "%s\n", n.Name)
		}
	}

	writeln(w, "--- all pods ---")
	allPods, err := kc.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, p := range allPods.Items {
			writef(w, "%-40s %-15s %s\n", p.Namespace+"/"+p.Name, string(p.Status.Phase), p.Status.PodIP)
		}
	}

	writeln(w, "--- flannel pod describes ---")
	flannelPods, err := kc.client.CoreV1().Pods(flannelNamespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, p := range flannelPods.Items {
			writef(w, "--- pod %s: phase=%s conditions=%v ---\n", p.Name, p.Status.Phase, p.Status.Conditions)
			for _, cs := range p.Status.ContainerStatuses {
				writef(w, "  container %s ready=%v restarts=%d state=%v\n", cs.Name, cs.Ready, cs.RestartCount, cs.State)
			}
		}
	}

	writeln(w, "--- flannel pod logs (current + previous) ---")
	if err == nil {
		for _, p := range flannelPods.Items {
			for _, previous := range []bool{false, true} {
				suffix := ""
				if previous {
					suffix = " (previous)"
				}
				logs, lerr := kc.client.CoreV1().Pods(flannelNamespace).
					GetLogs(p.Name, &corev1.PodLogOptions{Previous: previous}).DoRaw(ctx)
				if lerr == nil {
					writef(w, "--- pod %s%s ---\n%s\n", p.Name, suffix, logs)
				}
			}
		}
	}

	writeln(w, "--- flannel files on kind nodes ---")
	for _, node := range []string{kindControlPlane, kindWorker} {
		out, _ := kc.execOnNode(node, "ls", "-al", "/run/flannel")
		writef(w, "--- %s:/run/flannel ---\n%s\n", node, out)
		out, _ = kc.execOnNode(node, "cat", "/run/flannel/subnet.env")
		writef(w, "subnet.env:\n%s\n", out)
	}

	writeln(w, "--- flannel-related images on kind nodes ---")
	for _, node := range []string{kindControlPlane, kindWorker} {
		out, _ := kc.execOnNode(node, "crictl", "images")
		writef(w, "--- %s ---\n%s\n", node, out)
	}

	writeln(w, "--- events (all namespaces) ---")
	events, err := kc.client.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, e := range events.Items {
			writef(w, "%s  %s/%s  %s: %s\n",
				e.LastTimestamp.String(), e.Namespace, e.InvolvedObject.Name, e.Reason, e.Message)
		}
	}

	writeln(w, "======== END DEBUG INFO ========")
}
