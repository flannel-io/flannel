// +build !windows

// Copyright 2019 flannel authors
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

package ipsec

import (
	"fmt"
	"net"
	"syscall"

	log "github.com/golang/glog"
	"github.com/vishvananda/netlink"

	"github.com/coreos/flannel/pkg/ip"
)

type ipsecDeviceAttrs struct {
	name string
}

type ipsecDevice struct {
	link *netlink.Dummy
}

func newIPSecDummyDevice(devAttrs *ipsecDeviceAttrs) (*ipsecDevice, error) {
	link := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name: devAttrs.name,
		},
	}

	link, err := ensureLink(link)
	if err != nil {
		return nil, err
	}
	return &ipsecDevice{
		link: link,
	}, nil
}

func ensureLink(ipsec *netlink.Dummy) (*netlink.Dummy, error) {
	err := netlink.LinkAdd(ipsec)
	if err == syscall.EEXIST {
		// it's ok if the device already exists as long as config is similar
		log.V(1).Infof("IPSec device already exists")
		existing, err := netlink.LinkByName(ipsec.Name)
		if err != nil {
			return nil, err
		}

		// delete existing
		log.Warningf("%q already exists, recreating device", ipsec.Name)
		if err = netlink.LinkDel(existing); err != nil {
			return nil, fmt.Errorf("failed to delete interface: %v", err)
		}

		// create new
		if err = netlink.LinkAdd(ipsec); err != nil {
			return nil, fmt.Errorf("failed to create ipsec interface: %v", err)
		}
	} else if err != nil {
		return nil, err
	}

	ifindex := ipsec.Index
	link, err := netlink.LinkByIndex(ipsec.Index)
	if err != nil {
		return nil, fmt.Errorf("can't locate created ipsec device with index %v", ifindex)
	}

	var ok bool
	if ipsec, ok = link.(*netlink.Dummy); !ok {
		return nil, fmt.Errorf("created ipsec device with index %v is not dummy", ifindex)
	}

	return ipsec, nil
}

func (dev *ipsecDevice) Configure(ipn ip.IP4Net, cidr ip.IP4Net) error {
	if err := ip.EnsureV4AddressOnLink(ipn, dev.link); err != nil {
		return fmt.Errorf("failed to ensure address of interface %s: %s", dev.link.Attrs().Name, err)
	}

	if err := netlink.LinkSetUp(dev.link); err != nil {
		return fmt.Errorf("failed to set interface %s to UP state: %s", dev.link.Attrs().Name, err)
	}

	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.link.Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst: &net.IPNet{
			IP:   cidr.IP.ToIP(),
			Mask: net.CIDRMask(int(cidr.PrefixLen), 32),
		},
	}); err != nil {
		return fmt.Errorf("failed to add route to %v via interface %s: %s", cidr, dev.link.Attrs().Name, err)
	}

	return nil
}

func (dev *ipsecDevice) MACAddr() net.HardwareAddr {
	return dev.link.HardwareAddr
}
