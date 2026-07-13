//go:build windows

package subnet

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flannel-io/flannel/pkg/ip"
)

// renameio does not support Windows, and maintaining a full atomic write
// implementation for a platform without reliable atomic rename is not
// worth the complexity. Use a simple write instead.
func WriteSubnetFile(path string, config *Config, ipMasq bool, sn ip.IP4Net, ipv6sn ip.IP6Net, mtu int) error {
	dir, _ := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir subnet directory: %w", err)
	}

	var b bytes.Buffer
	if config.EnableIPv4 {
		fmt.Fprintf(&b, "FLANNEL_NETWORK=%s\n", config.Network)
		sn.IncrementIP()
		fmt.Fprintf(&b, "FLANNEL_SUBNET=%s\n", sn)
	}
	if config.EnableIPv6 {
		fmt.Fprintf(&b, "FLANNEL_IPV6_NETWORK=%s\n", config.IPv6Network)
		ipv6sn.IncrementIP()
		fmt.Fprintf(&b, "FLANNEL_IPV6_SUBNET=%s\n", ipv6sn)
	}

	fmt.Fprintf(&b, "FLANNEL_MTU=%d\n", mtu)
	fmt.Fprintf(&b, "FLANNEL_IPMASQ=%v\n", ipMasq)

	if err := os.WriteFile(path, b.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
