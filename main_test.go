package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/flannel-io/flannel/pkg/ip"
)

func TestReadCIDRsFromSubnetFileSkipsInvalidCIDRs(t *testing.T) {
	path := writeSubnetFile(t, "FLANNEL_SUBNET=10.244.1.0/24,not-a-cidr,10.244.2.0/24\n")

	defer assertNotPanics(t)()

	got := ReadCIDRsFromSubnetFile(path, "FLANNEL_SUBNET")
	want := []ip.IP4Net{
		mustParseIP4Net(t, "10.244.1.0/24"),
		mustParseIP4Net(t, "10.244.2.0/24"),
	}

	assertIP4NetsEqual(t, got, want)
}

func TestReadIP6CIDRsFromSubnetFileSkipsInvalidCIDRs(t *testing.T) {
	path := writeSubnetFile(t, "FLANNEL_IPV6_SUBNET=fd00::/64,not-a-cidr,fd01::/64\n")

	defer assertNotPanics(t)()

	got := ReadIP6CIDRsFromSubnetFile(path, "FLANNEL_IPV6_SUBNET")
	want := []ip.IP6Net{
		mustParseIP6Net(t, "fd00::/64"),
		mustParseIP6Net(t, "fd01::/64"),
	}

	assertIP6NetsEqual(t, got, want)
}

func TestReadCIDRFromSubnetFileInvalidOnlyReturnsEmpty(t *testing.T) {
	path := writeSubnetFile(t, "FLANNEL_SUBNET=not-a-cidr\n")

	defer assertNotPanics(t)()

	got := ReadCIDRFromSubnetFile(path, "FLANNEL_SUBNET")
	if !got.Empty() {
		t.Fatalf("expected empty ipv4 subnet, got %s", got)
	}
}

func TestReadIP6CIDRFromSubnetFileInvalidOnlyReturnsEmpty(t *testing.T) {
	path := writeSubnetFile(t, "FLANNEL_IPV6_SUBNET=not-a-cidr\n")

	defer assertNotPanics(t)()

	got := ReadIP6CIDRFromSubnetFile(path, "FLANNEL_IPV6_SUBNET")
	if !got.Empty() {
		t.Fatalf("expected empty ipv6 subnet, got %s", got)
	}
}

func writeSubnetFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "subnet.env")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write subnet file: %v", err)
	}

	return path
}

func assertNotPanics(t *testing.T) func() {
	t.Helper()

	return func() {
		t.Helper()
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}
}

func mustParseIP4Net(t *testing.T, cidr string) ip.IP4Net {
	t.Helper()

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse ipv4 cidr %q: %v", cidr, err)
	}

	return ip.FromIPNet(network)
}

func mustParseIP6Net(t *testing.T, cidr string) ip.IP6Net {
	t.Helper()

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse ipv6 cidr %q: %v", cidr, err)
	}

	return ip.FromIP6Net(network)
}

func assertIP4NetsEqual(t *testing.T, got, want []ip.IP4Net) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("unexpected ipv4 cidr count: got %d want %d", len(got), len(want))
	}

	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Fatalf("unexpected ipv4 cidr at index %d: got %s want %s", i, got[i], want[i])
		}
	}
}

func assertIP6NetsEqual(t *testing.T, got, want []ip.IP6Net) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("unexpected ipv6 cidr count: got %d want %d", len(got), len(want))
	}

	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Fatalf("unexpected ipv6 cidr at index %d: got %s want %s", i, got[i], want[i])
		}
	}
}
