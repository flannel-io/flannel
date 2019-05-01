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
// +build !windows

package ipsec

import (
	"net"
	"strconv"
	"sync"

	log "github.com/golang/glog"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

const (
	/*
	   New IP header (Tunnel Mode)   : 20
	   SPI (ESP Header)              : 4
	   Sequence (ESP Header)         : 4
	   ESP-AES IV                    : 16
	   ESP-AES Pad                   : 0-15
	   Pad length (ESP Trailer)      : 1
	   Next Header (ESP Trailer)     : 1
	   ESP-SHA-256 ICV               : 16
	*/
	ipsecOverhead    = 77
	udpEncapOverhead = 8

	defaultReqID = 11
)

type network struct {
	backend.SimpleNetwork
	password string
	UDPEncap bool
	sm       subnet.Manager
	iked     *CharonIKEDaemon
}

func newNetwork(sm subnet.Manager, extIface *backend.ExternalInterface,
	UDPEncap bool, password string, ikeDaemon *CharonIKEDaemon, l *subnet.Lease) (*network, error) {
	n := &network{
		SimpleNetwork: backend.SimpleNetwork{
			SubnetLease: l,
			ExtIface:    extIface,
		},
		sm:       sm,
		iked:     ikeDaemon,
		password: password,
		UDPEncap: UDPEncap,
	}

	return n, nil
}

func (n *network) Run(ctx context.Context) {

	err := n.iked.LoadSharedKey(n.SimpleNetwork.SubnetLease.Attrs.PublicIP.ToIP().String(), n.password)
	if err != nil {
		log.Errorf("Failed to load PSK: %v", err)
		return
	}

	wg := sync.WaitGroup{}
	defer wg.Wait()

	log.Info("Watching for new subnet leases")

	evts := make(chan []subnet.Event)

	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, n.sm, n.SubnetLease, evts)
		log.Info("WatchLeases exited")
		wg.Done()
	}()

	for {
		select {
		case evtsBatch := <-evts:
			log.Info("Handling event")
			n.handleSubnetEvents(evtsBatch)
		case <-ctx.Done():
			log.Info("Received DONE")
			return
		}
	}
}

func (n *network) handleSubnetEvents(batch []subnet.Event) {
	for _, evt := range batch {
		switch evt.Type {
		case subnet.EventAdded:
			log.Info("Subnet added: ", evt.Lease.Subnet)

			if evt.Lease.Attrs.BackendType != "ipsec" {
				log.Warningf("Ignoring non-ipsec event: type: %v", evt.Lease.Attrs.BackendType)
				continue
			}

			if evt.Lease.Subnet.Equal(n.SubnetLease.Subnet) {
				log.Warningf("Ignoring own lease add event: %+v", evt.Lease)
				continue
			}

			// We might want to delete only the policies we control and which need to be recreated.
			// However, for testing let's just recreate all.
			n.DeleteIPSECPolicies(
				n.SubnetLease.Subnet.ToIPNet(),
				evt.Lease.Subnet.ToIPNet(),
				n.SubnetLease.Attrs.PublicIP.ToIP(),
				evt.Lease.Attrs.PublicIP.ToIP(),
				defaultReqID,
			)

			n.AddIPSECPolicies(&evt.Lease, defaultReqID)

			if err := n.iked.LoadSharedKey(evt.Lease.Attrs.PublicIP.String(), n.password); err != nil {
				log.Errorf("error loading shared key into IKE daemon: %v", err)
			}

			if err := n.iked.LoadConnection(n.SubnetLease, &evt.Lease, strconv.Itoa(defaultReqID),
				strconv.FormatBool(n.UDPEncap)); err != nil {
				log.Errorf("error loading connection into IKE daemon: %v", err)
			}
			if err := n.AddRoute(&evt.Lease); err != nil {
				log.Errorf("failed to add route: %v", err)
			} else {
				log.Infof("added route for subnet.")
			}

		case subnet.EventRemoved:
			log.Info("Subnet removed: ", evt.Lease.Subnet)
			if evt.Lease.Attrs.BackendType != "ipsec" {
				log.Warningf("Ignoring non-ipsec event: type: %v", evt.Lease.Attrs.BackendType)
				continue
			}

			if evt.Lease.Subnet.Equal(n.SubnetLease.Subnet) {
				log.Warningf("Ignoring own lease remove event: %+v", evt.Lease)
				continue
			}

			if err := n.iked.UnloadCharonConnection(n.SubnetLease, &evt.Lease); err != nil {
				log.Errorf("error unloading charon connections: %v", err)
			}

			n.DeleteIPSECPolicies(
				n.SubnetLease.Subnet.ToIPNet(),
				evt.Lease.Subnet.ToIPNet(),
				n.SubnetLease.Attrs.PublicIP.ToIP(),
				evt.Lease.Attrs.PublicIP.ToIP(),
				defaultReqID,
			)

			if err := n.DelRoute(&evt.Lease); err != nil {
				log.Errorf("failed to add route to interface: %v", err)
			} else {
				log.Infof("deleted route for subnet.")
			}
		}
	}
}

func (n *network) MTU() int {
	mtu := n.ExtIface.Iface.MTU - ipsecOverhead
	if n.UDPEncap {
		mtu -= udpEncapOverhead
	}

	return mtu
}

func (n *network) AddIPSECPolicies(remoteLease *subnet.Lease, reqID int) {
	// We always want to try to add all policies to gracefully handle new policies.
	AddXFRMPolicy(n.SubnetLease, remoteLease, netlink.XFRM_DIR_OUT, reqID)
	AddXFRMPolicy(remoteLease, n.SubnetLease, netlink.XFRM_DIR_IN, reqID)
	AddXFRMPolicy(remoteLease, n.SubnetLease, netlink.XFRM_DIR_FWD, reqID)

	publicIPLease := &subnet.Lease{
		Attrs:      n.SubnetLease.Attrs,
		Asof:       n.SubnetLease.Asof,
		Expiration: n.SubnetLease.Expiration,
		Subnet: ip.IP4Net{
			IP:        n.SubnetLease.Attrs.PublicIP,
			PrefixLen: 32,
		},
	}
	AddXFRMPolicy(publicIPLease, remoteLease, netlink.XFRM_DIR_OUT, reqID)
	AddXFRMPolicy(remoteLease, publicIPLease, netlink.XFRM_DIR_IN, reqID)
	AddXFRMPolicy(remoteLease, publicIPLease, netlink.XFRM_DIR_FWD, reqID)

	remotePublicLease := &subnet.Lease{
		Attrs:      remoteLease.Attrs,
		Asof:       remoteLease.Asof,
		Expiration: remoteLease.Expiration,
		Subnet: ip.IP4Net{
			IP:        remoteLease.Attrs.PublicIP,
			PrefixLen: 32,
		},
	}
	AddXFRMPolicy(n.SubnetLease, remotePublicLease, netlink.XFRM_DIR_OUT, reqID)
	AddXFRMPolicy(remotePublicLease, n.SubnetLease, netlink.XFRM_DIR_IN, reqID)
	AddXFRMPolicy(remotePublicLease, n.SubnetLease, netlink.XFRM_DIR_FWD, reqID)
}

func (n *network) DeleteIPSECPolicies(localSubnet, remoteSubnet *net.IPNet, localPublicIP, remotePublicIP net.IP, reqID int) {
	DeleteXFRMPolicy(localSubnet, remoteSubnet, localPublicIP, remotePublicIP, netlink.XFRM_DIR_OUT, reqID)
	DeleteXFRMPolicy(remoteSubnet, localSubnet, remotePublicIP, localPublicIP, netlink.XFRM_DIR_IN, reqID)
	DeleteXFRMPolicy(remoteSubnet, localSubnet, remotePublicIP, localPublicIP, netlink.XFRM_DIR_FWD, reqID)

	publicSubnet := ip.IP4Net{
		IP:        ip.FromIP(localPublicIP),
		PrefixLen: 32,
	}.ToIPNet()

	remotePublicSubnet := ip.IP4Net{
		IP:        ip.FromIP(remotePublicIP),
		PrefixLen: 32,
	}.ToIPNet()

	DeleteXFRMPolicy(publicSubnet, remoteSubnet, localPublicIP, remotePublicIP, netlink.XFRM_DIR_OUT, reqID)
	DeleteXFRMPolicy(remoteSubnet, publicSubnet, remotePublicIP, localPublicIP, netlink.XFRM_DIR_IN, reqID)
	DeleteXFRMPolicy(remoteSubnet, publicSubnet, remotePublicIP, localPublicIP, netlink.XFRM_DIR_FWD, reqID)

	DeleteXFRMPolicy(remotePublicSubnet, localSubnet, localPublicIP, remotePublicIP, netlink.XFRM_DIR_OUT, reqID)
	DeleteXFRMPolicy(localSubnet, remotePublicSubnet, remotePublicIP, localPublicIP, netlink.XFRM_DIR_IN, reqID)
	DeleteXFRMPolicy(localSubnet, remotePublicSubnet, remotePublicIP, localPublicIP, netlink.XFRM_DIR_FWD, reqID)
}
