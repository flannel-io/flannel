// +build !windows

// Copyright 2017 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ipip

import (
	"encoding/json"
	"fmt"
	"syscall"

	"sync"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
	log "github.com/golang/glog"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"
)

const (
	backendType = "ipip"
	tunnelName  = "flannel.ipip"
)

func init() {
	backend.Register(backendType, New)
}

type IPIPBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := &IPIPBackend{
		sm:       sm,
		extIface: extIface,
	}
	return be, nil
}

func (be *IPIPBackend) RegisterNetwork(ctx context.Context, wg sync.WaitGroup, config *subnet.Config) (backend.Network, error) {
	cfg := struct {
		DirectRouting bool
	}{}

	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding IPIP backend config: %v", err)
		}
	}

	log.Infof("IPIP config: DirectRouting=%v", cfg.DirectRouting)

	n := &backend.RouteNetwork{
		SimpleNetwork: backend.SimpleNetwork{
			ExtIface: be.extIface,
		},
		SM:          be.sm,
		BackendType: backendType,
	}

	attrs := &subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(be.extIface.ExtAddr),
		BackendType: backendType,
	}

	l, err := be.sm.AcquireLease(ctx, attrs)
	switch err {
	case nil:
		n.SubnetLease = l
	case context.Canceled, context.DeadlineExceeded:
		return nil, err
	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	link, err := be.configureIPIPDevice(n.SubnetLease)

	if err != nil {
		return nil, err
	}

	n.Mtu = link.MTU
	n.LinkIndex = link.Index
	n.GetRoute = func(lease *subnet.Lease) *netlink.Route {
		route := netlink.Route{
			Dst:       lease.Subnet.ToIPNet(),
			Gw:        lease.Attrs.PublicIP.ToIP(),
			LinkIndex: n.LinkIndex,
			Flags:     int(netlink.FLAG_ONLINK),
		}

		if cfg.DirectRouting {
			dr, err := ip.DirectRouting(lease.Attrs.PublicIP.ToIP())

			if err != nil {
				log.Error(err)
			}

			if dr {
				log.V(2).Infof("configure route to %v via direct routing", lease.Attrs.PublicIP.String())
				route.LinkIndex = n.ExtIface.Iface.Index
			}
		}

		return &route
	}

	return n, nil
}

func (be *IPIPBackend) configureIPIPDevice(lease *subnet.Lease) (*netlink.Iptun, error) {
	// When modprobe ipip module, a tunl0 ipip device is created automatically per network namespace by ipip kernel module.
	// It is the namespace default IPIP device with attributes local=any and remote=any.
	// When receiving IPIP protocol packets, kernel will forward them to tunl0 as a fallback device
	// if it can't find an option whose local/remote attribute matches their src/dst ip address more precisely.
	// See https://github.com/torvalds/linux/blob/v4.13/net/ipv4/ip_tunnel.c#L85-L95 .

	// So we have two options of creating ipip device, either rename tunl0 to flannel.ipip or create an new ipip device
	// and set local attribute of flannel.ipip to distinguish these two devices.
	// Considering tunl0 might be used by users, so choose the later option.
	link := &netlink.Iptun{LinkAttrs: netlink.LinkAttrs{Name: tunnelName}, Local: be.extIface.IfaceAddr}

	if err := netlink.LinkAdd(link); err != nil {
		if err != syscall.EEXIST {
			return nil, err
		}

		// The link already exists, so check existing link attributes.
		existing, err := netlink.LinkByName(tunnelName)
		if err != nil {
			return nil, err
		}

		// If there's an exists device but it's not an ipip/IpTun device then get the user to fix it (flannel shouldn't
		// delete a user's device)
		if existing.Type() != "ipip" {
			return nil, fmt.Errorf("%v isn't an ipip mode device, please remove device and try again", tunnelName)
		}
		ipip, ok := existing.(*netlink.Iptun)
		if !ok {
			return nil, fmt.Errorf("%s isn't an iptun device (%#v), please remove device and try again", tunnelName, link)
		}

		// local attribute may change if a user changes iface configuration, we need to recreate the device to ensure
		// local and remote attribute is expected.
		// local should be equal to the extIface.IfaceAddr and remote should be nil (or equal to 0.0.0.0)
		if ipip.Local == nil || !ipip.Local.Equal(be.extIface.IfaceAddr) || (ipip.Remote != nil && ipip.Remote.String() != "0.0.0.0") {
			log.Warningf("%q already exists with incompatable attributes: local=%v remote=%v; recreating device",
				tunnelName, ipip.Local, ipip.Remote)

			if err = netlink.LinkDel(existing); err != nil {
				return nil, fmt.Errorf("failed to delete interface: %v", err)
			}

			if err = netlink.LinkAdd(link); err != nil {
				return nil, fmt.Errorf("failed to create ipip interface: %v", err)
			}
		}
	}

	// Due to the extra 20 byte IP header that the tunnel will add to each packet,
	// MTU size for both the workload and tunnel interfaces should be 20 bytes less than the selected iface (specified with the --iface option).
	expectMTU := be.extIface.Iface.MTU - 20
	if expectMTU <= 0 {
		return nil, fmt.Errorf("MTU %d of iface %s is too small for ipip mode to work", be.extIface.Iface.MTU, be.extIface.Iface.Name)
	}

	oldMTU := link.Attrs().MTU
	if oldMTU > expectMTU || oldMTU == 0 {
		log.Infof("current MTU of %s is %d, setting it to %d", tunnelName, oldMTU, expectMTU)
		err := netlink.LinkSetMTU(link, expectMTU)

		if err != nil {
			return nil, fmt.Errorf("failed to set %v MTU to %d: %v", tunnelName, expectMTU, err)
		}
		// change MTU as it will be written into /run/flannel/subnet.env
		link.Attrs().MTU = expectMTU
	}

	// Ensure that the device has a /32 address so that no broadcast routes are created.
	// This IP is just used as a source address for host to workload traffic (so
	// the return path for the traffic has an address on the flannel network to use as the destination)
	if err := ip.EnsureV4AddressOnLink(ip.IP4Net{IP: lease.Subnet.IP, PrefixLen: 32}, link); err != nil {
		return nil, fmt.Errorf("failed to ensure address of interface %s: %s", link.Attrs().Name, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return nil, fmt.Errorf("failed to set %v UP: %v", tunnelName, err)
	}

	return link, nil
}
