//go:build !windows

package subnet

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/google/renameio/v2"
)

// WriteSubnetFile atomically writes the flannel subnet configuration file.
// Uses google/renameio for safe atomic write semantics, which handles creating
// a temporary file, syncing, closing, renaming, and directory fsync.
func WriteSubnetFile(path string, config *Config, ipMasq bool, sn ip.IP4Net, ipv6sn ip.IP6Net, mtu int) error {
	dir, _ := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir subnet directory: %w", err)
	}

	// Preserve original file permissions if the file already exists
	perm := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	f, err := renameio.TempFile(dir, path)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer f.Cleanup()

	if err := f.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if config.EnableIPv4 {
		// Save the CIDR assigned to flannel
		if _, err := fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", config.Network); err != nil {
			return fmt.Errorf("failed to write FLANNEL_NETWORK: %w", err)
		}
		// Write out the first usable IP by incrementing sn.IP by one
		sn.IncrementIP()
		if _, err := fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn); err != nil {
			return fmt.Errorf("failed to write FLANNEL_SUBNET: %w", err)
		}
	}
	if config.EnableIPv6 {
		// Save the CIDR assigned to flannel
		if _, err := fmt.Fprintf(f, "FLANNEL_IPV6_NETWORK=%s\n", config.IPv6Network); err != nil {
			return fmt.Errorf("failed to write FLANNEL_IPV6_NETWORK: %w", err)
		}
		// Write out the first usable IP by incrementing ip6Sn.IP by one
		ipv6sn.IncrementIP()
		if _, err := fmt.Fprintf(f, "FLANNEL_IPV6_SUBNET=%s\n", ipv6sn); err != nil {
			return fmt.Errorf("failed to write FLANNEL_IPV6_SUBNET: %w", err)
		}
	}

	if _, err := fmt.Fprintf(f, "FLANNEL_MTU=%d\n", mtu); err != nil {
		return fmt.Errorf("failed to write FLANNEL_MTU: %w", err)
	}
	if _, err := fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq); err != nil {
		return fmt.Errorf("failed to write FLANNEL_IPMASQ: %w", err)
	}

	return f.CloseAtomicallyReplace()
}
