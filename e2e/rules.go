//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// expectedIptablesPostrouting builds the golden FLANNEL-POSTRTG rule set for a
// node with the given pod CIDR, mirroring check_iptables.
func expectedIptablesPostrouting(podCIDR string) string {
	return strings.Join([]string{
		`-A POSTROUTING -m comment --comment "flanneld masq" -j FLANNEL-POSTRTG`,
		`-N FLANNEL-POSTRTG`,
		`-A FLANNEL-POSTRTG -m mark --mark 0x4000/0x4000 -m comment --comment "flanneld masq" -j RETURN`,
		fmt.Sprintf(`-A FLANNEL-POSTRTG -s %s -d %s -m comment --comment "flanneld masq" -j RETURN`, podCIDR, flannelNet),
		fmt.Sprintf(`-A FLANNEL-POSTRTG -s %s -d %s -m comment --comment "flanneld masq" -j RETURN`, flannelNet, podCIDR),
		fmt.Sprintf(`-A FLANNEL-POSTRTG ! -s %s -d %s -m comment --comment "flanneld masq" -j RETURN`, flannelNet, podCIDR),
		fmt.Sprintf(`-A FLANNEL-POSTRTG -s %s ! -d 224.0.0.0/4 -m comment --comment "flanneld masq" -j MASQUERADE --random-fully`, flannelNet),
		fmt.Sprintf(`-A FLANNEL-POSTRTG ! -s %s -d %s -m comment --comment "flanneld masq" -j MASQUERADE --random-fully`, flannelNet, flannelNet),
	}, "\n")
}

func expectedIptablesForward() string {
	return strings.Join([]string{
		`-P FORWARD ACCEPT`,
		`-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes load balancer firewall" -j KUBE-PROXY-FIREWALL`,
		`-A FORWARD -m comment --comment "kubernetes forwarding rules" -j KUBE-FORWARD`,
		`-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes service portals" -j KUBE-SERVICES`,
		`-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes externally-visible service portals" -j KUBE-EXTERNAL-SERVICES`,
		`-A FORWARD -m comment --comment "flanneld forward" -j FLANNEL-FWD`,
		`-N FLANNEL-FWD`,
		fmt.Sprintf(`-A FLANNEL-FWD -s %s -m comment --comment "flanneld forward" -j ACCEPT`, flannelNet),
		fmt.Sprintf(`-A FLANNEL-FWD -d %s -m comment --comment "flanneld forward" -j ACCEPT`, flannelNet),
	}, "\n")
}

// checkIptables asserts the masquerade and forward rules on both nodes, mirroring
// the bash check_iptables helper.
func (kc *kindCluster) checkIptables(ctx context.Context) {
	workerCIDR, err := kc.podCIDR(ctx, kindWorker)
	Expect(err).NotTo(HaveOccurred())
	leaderCIDR, err := kc.podCIDR(ctx, kindControlPlane)
	Expect(err).NotTo(HaveOccurred())

	for node, cidr := range map[string]string{kindWorker: workerCIDR, kindControlPlane: leaderCIDR} {
		post := kc.iptablesNatFlannel(node)
		Expect(post).To(Equal(expectedIptablesPostrouting(cidr)),
			"node %s has unexpected postrouting rules", node)

		fwd := kc.iptablesFilterForward(node)
		Expect(fwd).To(Equal(expectedIptablesForward()),
			"node %s has unexpected forward rules", node)
	}
}

func (kc *kindCluster) iptablesNatFlannel(node string) string {
	post, err := kc.execOnNode(node, "/usr/sbin/iptables", "-t", "nat", "-S", "POSTROUTING")
	Expect(err).NotTo(HaveOccurred())
	chain, err := kc.execOnNode(node, "/usr/sbin/iptables", "-t", "nat", "-S", "FLANNEL-POSTRTG")
	Expect(err).NotTo(HaveOccurred())

	var lines []string
	for _, l := range strings.Split(strings.TrimRight(post, "\n"), "\n") {
		if strings.Contains(l, "FLANNEL") {
			lines = append(lines, l)
		}
	}
	lines = append(lines, strings.Split(strings.TrimRight(chain, "\n"), "\n")...)
	return strings.Join(lines, "\n")
}

func (kc *kindCluster) iptablesFilterForward(node string) string {
	fwd, err := kc.execOnNode(node, "/usr/sbin/iptables", "-t", "filter", "-S", "FORWARD")
	Expect(err).NotTo(HaveOccurred())
	chain, err := kc.execOnNode(node, "/usr/sbin/iptables", "-t", "filter", "-S", "FLANNEL-FWD")
	Expect(err).NotTo(HaveOccurred())
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
	workerCIDR, err := kc.podCIDR(ctx, kindWorker)
	Expect(err).NotTo(HaveOccurred())
	leaderCIDR, err := kc.podCIDR(ctx, kindControlPlane)
	Expect(err).NotTo(HaveOccurred())

	for node, cidr := range map[string]string{kindWorker: workerCIDR, kindControlPlane: leaderCIDR} {
		post, err := kc.execOnNode(node, "/usr/sbin/nft", "list", "chain", "flannel-ipv4", "postrtg")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimRight(post, "\n")).To(Equal(expectedNftPostrouting(cidr)),
			"node %s has unexpected nftables postrouting rules", node)

		fwd, err := kc.execOnNode(node, "/usr/sbin/nft", "list", "chain", "flannel-ipv4", "forward")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimRight(fwd, "\n")).To(Equal(expectedNftForward()),
			"node %s has unexpected nftables forward rules", node)
	}
}

// dumpDebugInfo prints cluster state for troubleshooting, mirroring
// dump_debug_info. It is invoked from AfterEach on failure.
func (kc *kindCluster) dumpDebugInfo(ctx context.Context) {
	if kc == nil || kc.client == nil {
		return
	}
	w := GinkgoWriter
	fmt.Fprintln(w, "======== DEBUG INFO ========")

	nodes, err := kc.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		fmt.Fprintln(w, "--- nodes ---")
		for _, n := range nodes.Items {
			fmt.Fprintf(w, "%s\n", n.Name)
		}
	}

	fmt.Fprintln(w, "--- flannel pod logs ---")
	pods, err := kc.client.CoreV1().Pods(flannelNamespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, p := range pods.Items {
			fmt.Fprintf(w, "--- pod %s ---\n", p.Name)
			logs, lerr := kc.client.CoreV1().Pods(flannelNamespace).
				GetLogs(p.Name, &corev1.PodLogOptions{}).DoRaw(ctx)
			if lerr == nil {
				fmt.Fprintln(w, string(logs))
			}
		}
	}

	fmt.Fprintln(w, "--- flannel files on kind nodes ---")
	for _, node := range []string{kindControlPlane, kindWorker} {
		out, _ := kc.execOnNode(node, "cat", "/run/flannel/subnet.env")
		fmt.Fprintf(w, "--- %s:/run/flannel/subnet.env ---\n%s\n", node, out)
	}
	fmt.Fprintln(w, "======== END DEBUG INFO ========")
}
