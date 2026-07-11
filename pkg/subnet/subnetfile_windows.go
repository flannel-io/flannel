//go:build windows

package subnet

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/flannel-io/flannel/pkg/ip"
	"golang.org/x/sys/windows"
)

// WriteSubnetFile atomically writes the flannel subnet configuration file.
// Windows does not provide a reliable atomic file replacement syscall, so this
// is a best-effort approximation. The file is written to a temporary location
// on the same directory, synced, then moved into place via MoveFileEx with
// MOVEFILE_WRITE_THROUGH to flush metadata as much as the system allows.
func WriteSubnetFile(path string, config *Config, ipMasq bool, sn ip.IP4Net, ipv6sn ip.IP6Net, mtu int) error {
	dir, name := filepath.Split(path)
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

	// Uses a dotted-prefix pattern (".subnet.env.") that matches
	// what renameio generates internally on Unix.
	f, err := os.CreateTemp(dir, "."+name+".")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	// On early exit (before the move), remove the temp file.
	// After a successful MoveFileEx the source path no longer exists,
	// so this becomes a harmless no-op.
	defer os.Remove(f.Name())

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

	// 1. Write
	if _, err := fmt.Fprintf(f, "FLANNEL_MTU=%d\n", mtu); err != nil {
		return fmt.Errorf("failed to write FLANNEL_MTU: %w", err)
	}
	if _, err := fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq); err != nil {
		return fmt.Errorf("failed to write FLANNEL_IPMASQ: %w", err)
	}

	// 2. Flush file contents
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("failed to sync subnet file: %w", err)
	}

	// 3. Close the file
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close subnet file: %w", err)
	}

	src, err := windows.UTF16PtrFromString(f.Name())
	if err != nil {
		return fmt.Errorf("failed to encode source path: %w", err)
	}

	dst, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("failed to encode destination path: %w", err)
	}

	// 4. Atomically swap the temp file into place.
	// MOVEFILE_REPLACE_EXISTING overwrites the destination if present.
	// MOVEFILE_WRITE_THROUGH requests the move and its metadata be flushed
	// to persistent storage before returning, as close to fsync(dir) as
	// Windows supports for this operation.
	return windows.MoveFileEx(
		src,
		dst,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}
