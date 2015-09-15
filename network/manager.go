// Copyright 2015 CoreOS, Inc.
// Copyright 2015 Red Hat, Inc.
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

package network

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/go-systemd/daemon"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

type CmdLineOpts struct {
	publicIP      string
	ipMasq        bool
	subnetFile    string
	subnetDir     string
	iface         string
	networks      string
	watchNetworks bool
}

var opts CmdLineOpts

func init() {
	flag.StringVar(&opts.publicIP, "public-ip", "", "IP accessible by other nodes for inter-host communication")
	flag.StringVar(&opts.subnetFile, "subnet-file", "/run/flannel/subnet.env", "filename where env variables (subnet, MTU, ... ) will be written to")
	flag.StringVar(&opts.subnetDir, "subnet-dir", "/run/flannel/networks", "directory where files with env variables (subnet, MTU, ...) will be written to")
	flag.StringVar(&opts.iface, "iface", "", "interface to use (IP or name) for inter-host communication")
	flag.StringVar(&opts.networks, "networks", "", "run in multi-network mode and service the specified networks")
	flag.BoolVar(&opts.watchNetworks, "watch-networks", false, "run in multi-network mode and watch for networks from 'networks' or all networks")
	flag.BoolVar(&opts.ipMasq, "ip-masq", false, "setup IP masquerade rule for traffic destined outside of overlay network")
}

type Manager struct {
	ctx             context.Context
	sm              subnet.Manager
	allowedNetworks map[string]bool
	networks        map[string]*Network
	watch           bool
	ipMasq          bool
	extIface        *net.Interface
	iaddr           net.IP
	eaddr           net.IP
}

func (m *Manager) isNetAllowed(name string) bool {
	// If allowedNetworks is empty all networks are allowed
	if len(m.allowedNetworks) > 0 {
		_, ok := m.allowedNetworks[name]
		return ok
	}
	return true
}

func (m *Manager) isMultiNetwork() bool {
	return len(m.allowedNetworks) > 0 || m.watch
}

func NewNetworkManager(ctx context.Context, sm subnet.Manager) (*Manager, error) {
	iface, iaddr, err := lookupExtIface(opts.iface)
	if err != nil {
		return nil, err
	}

	if iface.MTU == 0 {
		return nil, fmt.Errorf("Failed to determine MTU for %s interface", iaddr)
	}

	var eaddr net.IP

	if len(opts.publicIP) > 0 {
		eaddr = net.ParseIP(opts.publicIP)
		if eaddr == nil {
			return nil, fmt.Errorf("Invalid public IP address", opts.publicIP)
		}
	}

	if eaddr == nil {
		eaddr = iaddr
	}

	log.Infof("Using %s as external interface", iaddr)
	log.Infof("Using %s as external endpoint", eaddr)

	manager := &Manager{
		ctx:             ctx,
		sm:              sm,
		allowedNetworks: make(map[string]bool),
		networks:        make(map[string]*Network),
		watch:           opts.watchNetworks,
		ipMasq:          opts.ipMasq,
		extIface:        iface,
		iaddr:           iaddr,
		eaddr:           eaddr,
	}

	for _, name := range strings.Split(opts.networks, ",") {
		if name != "" {
			manager.allowedNetworks[name] = true
		}
	}

	if manager.isMultiNetwork() {
		// Get list of existing networks
		result, err := manager.sm.WatchNetworks(ctx, nil)
		if err != nil {
			return nil, err
		}

		for _, n := range result.Snapshot {
			if manager.isNetAllowed(n) {
				manager.networks[n] = NewNetwork(ctx, sm, n, manager.ipMasq)
			}
		}
	} else {
		manager.networks[""] = NewNetwork(ctx, sm, "", manager.ipMasq)
	}

	return manager, nil
}

func lookupExtIface(ifname string) (*net.Interface, net.IP, error) {
	var iface *net.Interface
	var iaddr net.IP
	var err error

	if len(ifname) > 0 {
		if iaddr = net.ParseIP(ifname); iaddr != nil {
			iface, err = ip.GetInterfaceByIP(iaddr)
			if err != nil {
				return nil, nil, fmt.Errorf("Error looking up interface %s: %s", ifname, err)
			}
		} else {
			iface, err = net.InterfaceByName(ifname)
			if err != nil {
				return nil, nil, fmt.Errorf("Error looking up interface %s: %s", ifname, err)
			}
		}
	} else {
		log.Info("Determining IP address of default interface")
		if iface, err = ip.GetDefaultGatewayIface(); err != nil {
			return nil, nil, fmt.Errorf("Failed to get default interface: %s", err)
		}
	}

	if iaddr == nil {
		iaddr, err = ip.GetIfaceIP4Addr(iface)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to find IPv4 address for interface %s", iface.Name)
		}
	}

	return iface, iaddr, nil
}

func writeSubnetFile(path string, nw ip.IP4Net, ipMasq bool, sd *backend.SubnetDef) error {
	dir, name := filepath.Split(path)
	os.MkdirAll(dir, 0755)

	tempFile := filepath.Join(dir, "."+name)
	f, err := os.Create(tempFile)
	if err != nil {
		return err
	}

	// Write out the first usable IP by incrementing
	// sn.IP by one
	sn := sd.Lease.Subnet
	sn.IP += 1

	fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", nw)
	fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn)
	fmt.Fprintf(f, "FLANNEL_MTU=%d\n", sd.MTU)
	_, err = fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq)
	f.Close()
	if err != nil {
		return err
	}

	// rename(2) the temporary file to the desired location so that it becomes
	// atomically visible with the contents
	return os.Rename(tempFile, path)
}

func (m *Manager) RunNetwork(net *Network) {
	sn := net.Init(m.extIface, m.iaddr, m.eaddr)
	if sn != nil {
		if m.isMultiNetwork() {
			path := filepath.Join(opts.subnetDir, net.Name) + ".env"
			if err := writeSubnetFile(path, net.Config.Network, m.ipMasq, sn); err != nil {
				log.Warningf("%v failed to write subnet file: %s", net.Name, err)
				return
			}
		} else {
			if err := writeSubnetFile(opts.subnetFile, net.Config.Network, m.ipMasq, sn); err != nil {
				log.Warningf("%v failed to write subnet file: %s", net.Name, err)
				return
			}
			daemon.SdNotify("READY=1")
		}

		log.Infof("Running network %v", net.Name)
		net.Run()
		log.Infof("%v exited", net.Name)
	}
}

func (m *Manager) watchNetworks() {
	wg := sync.WaitGroup{}

	events := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchNetworks(m.ctx, m.sm, events)
		wg.Done()
	}()
	// skip over the initial snapshot
	<-events

	for {
		select {
		case <-m.ctx.Done():
			break

		case evtBatch := <-events:
			for _, e := range evtBatch {
				netname := e.Network
				if !m.isNetAllowed(netname) {
					continue
				}

				switch e.Type {
				case subnet.EventAdded:
					if _, ok := m.networks[netname]; ok {
						continue
					}
					net := NewNetwork(m.ctx, m.sm, netname, m.ipMasq)
					m.networks[netname] = net
					wg.Add(1)
					go func() {
						m.RunNetwork(net)
						wg.Done()
					}()

				case subnet.EventRemoved:
					net, ok := m.networks[netname]
					if !ok {
						log.Warningf("Network %v unknown; ignoring EventRemoved", netname)
						continue
					}
					net.Cancel()
					delete(m.networks, netname)
				}
			}
		}
	}

	wg.Wait()
}

func (m *Manager) Run() {
	wg := sync.WaitGroup{}

	// Run existing networks
	for _, net := range m.networks {
		wg.Add(1)
		go func(n *Network) {
			m.RunNetwork(n)
			wg.Done()
		}(net)
	}

	if opts.watchNetworks {
		wg.Add(1)
		go func() {
			m.watchNetworks()
			wg.Done()
		}()
	} else {
		<-m.ctx.Done()
	}

	wg.Wait()
}
