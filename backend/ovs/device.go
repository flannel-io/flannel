// Copyright 2015 CoreOS, Inc.
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

package ovs

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/vishvananda/netlink"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/vishvananda/netlink/nl"

	"github.com/coreos/flannel/pkg/ip"
)

type ovsDevice struct {
	bridgeName      string
	oflowProto      string
	opHandler       OpHandler
	clusterNetwork  ip.IP4Net
	nodeNetwork     ip.IP4Net
	servicesNetwork ip.IP4Net
	gwAddr          net.IP
}

func newOVSDevice() (*ovsDevice, error) {
	return newOVSDeviceWithHandler(&liveOpHandler{})
}

func newOVSDeviceWithHandler(handler OpHandler) (*ovsDevice, error) {
	bridge := &ovsDevice{
		bridgeName: "br0",
		oflowProto: "OpenFlow13",
		opHandler:  handler,
	}
	err := bridge.genericSetup()
	return bridge, err
}

//*****************************************************************************

type OpHandler interface {
	OvsExec(cmd string, args ...string) ([]byte, error)
	Modprobe(module string) ([]byte, error)
	LinkAdd(link netlink.Link) error
	LinkDel(link netlink.Link) error
	LinkByIndex(index int) (netlink.Link, error)
	LinkByName(name string) (netlink.Link, error)
	LinkSetUp(link netlink.Link) error
	LinkSetMaster(link netlink.Link, master *netlink.Bridge) error
	AddrAdd(link netlink.Link, addr *netlink.Addr) error
	AddrList(link netlink.Link, family int) ([]netlink.Addr, error)
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	RouteAdd(route *netlink.Route) error
	RouteDel(route *netlink.Route) error
	RouteDelWithProtocol(route *netlink.Route, protocol uint8) error
	Sysctl(item, value string) error
}

type liveOpHandler struct{}

func (h *liveOpHandler) OvsExec(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).CombinedOutput()
}

func (h *liveOpHandler) Modprobe(module string) ([]byte, error) {
	return exec.Command("modprobe", module).CombinedOutput()
}

func (h *liveOpHandler) LinkAdd(link netlink.Link) error {
	return netlink.LinkAdd(link)
}

func (h *liveOpHandler) LinkDel(link netlink.Link) error {
	return netlink.LinkDel(link)
}

func (h *liveOpHandler) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

func (h *liveOpHandler) LinkByIndex(index int) (netlink.Link, error) {
	return netlink.LinkByIndex(index)
}

func (h *liveOpHandler) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

func (h *liveOpHandler) LinkSetMaster(link netlink.Link, master *netlink.Bridge) error {
	return netlink.LinkSetMaster(link, master)
}

func (h *liveOpHandler) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrAdd(link, addr)
}

func (h *liveOpHandler) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	return netlink.AddrList(link, family)
}

func (h *liveOpHandler) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	return netlink.RouteList(link, family)
}

func (h *liveOpHandler) RouteAdd(route *netlink.Route) error {
	return netlink.RouteAdd(route)
}

func (h *liveOpHandler) RouteDel(route *netlink.Route) error {
	return netlink.RouteDel(route)
}

func (h *liveOpHandler) RouteDelWithProtocol(route *netlink.Route, protocol uint8) error {
	if (route.Dst == nil || route.Dst.IP == nil) && route.Src == nil && route.Gw == nil {
		return fmt.Errorf("one of Dst.IP, Src, or Gw must not be nil")
	}

	req := nl.NewNetlinkRequest(syscall.RTM_DELROUTE, syscall.NLM_F_ACK)

	msg := nl.NewRtMsg()
	// vishvananda/netlink hardcodes Protocol to RTPROT_BOOT; override that
	msg.Protocol = syscall.RTPROT_UNSPEC
	msg.Scope = uint8(route.Scope)
	family := -1
	var rtAttrs []*nl.RtAttr

	if route.Dst != nil && route.Dst.IP != nil {
		dstLen, _ := route.Dst.Mask.Size()
		msg.Dst_len = uint8(dstLen)
		dstFamily := nl.GetIPFamily(route.Dst.IP)
		family = dstFamily
		var dstData []byte
		if dstFamily == netlink.FAMILY_V4 {
			dstData = route.Dst.IP.To4()
		} else {
			dstData = route.Dst.IP.To16()
		}
		rtAttrs = append(rtAttrs, nl.NewRtAttr(syscall.RTA_DST, dstData))
	}

	if route.Src != nil {
		srcFamily := nl.GetIPFamily(route.Src)
		if family != -1 && family != srcFamily {
			return fmt.Errorf("source and destination ip are not the same IP family")
		}
		family = srcFamily
		var srcData []byte
		if srcFamily == netlink.FAMILY_V4 {
			srcData = route.Src.To4()
		} else {
			srcData = route.Src.To16()
		}
		// The commonly used src ip for routes is actually PREFSRC
		rtAttrs = append(rtAttrs, nl.NewRtAttr(syscall.RTA_PREFSRC, srcData))
	}

	if route.Gw != nil {
		gwFamily := nl.GetIPFamily(route.Gw)
		if family != -1 && family != gwFamily {
			return fmt.Errorf("gateway, source, and destination ip are not the same IP family")
		}
		family = gwFamily
		var gwData []byte
		if gwFamily == netlink.FAMILY_V4 {
			gwData = route.Gw.To4()
		} else {
			gwData = route.Gw.To16()
		}
		rtAttrs = append(rtAttrs, nl.NewRtAttr(syscall.RTA_GATEWAY, gwData))
	}

	msg.Family = uint8(family)

	req.AddData(msg)
	for _, attr := range rtAttrs {
		req.AddData(attr)
	}

	var (
		b      = make([]byte, 4)
		native = nl.NativeEndian()
	)
	native.PutUint32(b, uint32(route.LinkIndex))

	req.AddData(nl.NewRtAttr(syscall.RTA_OIF, b))

	_, err := req.Execute(syscall.NETLINK_ROUTE, 0)
	return err
}

func (h *liveOpHandler) Sysctl(item, value string) error {
	o, err := exec.Command("sysctl", "-w", fmt.Sprintf("%s=%s", item, value)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysctl %s=%s failed: %s (%o)", item, value, err, o)
	}
	return nil
}

//*****************************************************************************

func (dev *ovsDevice) vsCtl(cmd string, args ...string) error {
	allArgs := make([]string, 0, 1+len(args))
	allArgs = append(allArgs, cmd)
	for _, a := range args {
		allArgs = append(allArgs, a)
	}
	_, err := dev.opHandler.OvsExec("ovs-vsctl", allArgs...)
	return err
}

func (dev *ovsDevice) ofCtl(cmd, theRest string) ([]byte, error) {
	return dev.opHandler.OvsExec("ovs-ofctl", "-O", dev.oflowProto, cmd, dev.bridgeName, theRest)
}

func (dev *ovsDevice) addFlow(flow string, args ...interface{}) error {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}
	log.Infof("OVS ADD FLOW: %s", flow)
	out, err := dev.ofCtl("add-flow", flow)
	if err != nil {
		log.Errorf("Error adding OVS flow %s: %s (%v)", flow, out, err)
		return err
	}
	return nil
}

func (dev *ovsDevice) delFlows(flow string, args ...interface{}) error {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}
	out, err := dev.ofCtl("del-flows", flow)
	log.Infof("Output of deleting network %s flows: %s (%v)", flow, out, err)
	return err
}

func (dev *ovsDevice) getPortNumber(portName string) (uint, error) {
	out, err := dev.ofCtl("dump-ports", portName)
	if err != nil {
		return 0, err
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "port") {
			continue
		}
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		u, err := strconv.ParseUint(strings.TrimSpace(line[4:colon]), 10, 32)
		if err != nil {
			return 0, err
		}
		return uint(u), nil
	}
	return 0, fmt.Errorf("Failed to find OVS port number for %s", portName)
}

//*****************************************************************************

func (dev *ovsDevice) addGatewayAddressToLink(linkName string) (netlink.Link, error) {
	link, err := dev.opHandler.LinkByName(linkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %v", linkName, err)
	}
	addrStr := fmt.Sprintf("%s/%d", dev.gwAddr.String(), dev.nodeNetwork.PrefixLen)
	addr, err := netlink.ParseAddr(addrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gateway address %s: %v", addrStr, err)
	}
	if err := dev.opHandler.AddrAdd(link, addr); err != nil {
		return nil, fmt.Errorf("failed to add gateway address %s to %s: %v", linkName, addrStr, err)
	}
	if err := dev.opHandler.LinkSetUp(link); err != nil {
		return nil, fmt.Errorf("failed to bring up %s: %v", linkName, err)
	}

	return link, nil
}

func (dev *ovsDevice) matchRoute(m syscall.NetlinkMessage, route *netlink.Route) bool {
	if m.Header.Type != syscall.RTM_NEWROUTE {
		return false
	}

	msg := nl.DeserializeRtMsg(m.Data)

	if msg.Flags&syscall.RTM_F_CLONED != 0 {
		return false
	}
	if msg.Table != syscall.RT_TABLE_LOCAL && msg.Table != syscall.RT_TABLE_MAIN {
		return false
	}
	if msg.Scope != uint8(route.Scope) {
		return false
	}

	attrs, err := nl.ParseRouteAttr(m.Data[msg.Len():])
	if err != nil {
		return false
	}

	var gotGw, gotSrc, gotDst, gotIfindex bool
	native := nl.NativeEndian()
	for _, attr := range attrs {
		switch attr.Attr.Type {
		case syscall.RTA_GATEWAY:
			// not always present
			if !net.IP(attr.Value).Equal(route.Gw) {
				return false
			}
			gotGw = true
		case syscall.RTA_PREFSRC:
			if !net.IP(attr.Value).Equal(route.Src) {
				return false
			}
			gotSrc = true
		case syscall.RTA_DST:
			dst := &net.IPNet{
				IP:   attr.Value,
				Mask: net.CIDRMask(int(msg.Dst_len), 8*len(attr.Value)),
			}
			if !dst.IP.Equal(route.Dst.IP) {
				return false
			}
			if !bytes.Equal(dst.Mask, route.Dst.Mask) {
				return false
			}
			gotDst = true
		case syscall.RTA_OIF:
			routeIndex := int(native.Uint32(attr.Value[0:4]))
			if routeIndex != route.LinkIndex {
				return false
			}
			gotIfindex = true
		}
	}

	// RTA_GATEWAY not always present if empty
	if route.Gw != nil && !gotGw {
		return false
	}

	return gotSrc && gotDst && gotIfindex
}

func (dev *ovsDevice) removeLbrSubnetRoute(nlsock *nl.NetlinkSocket, ifindex int) {
	// Remove the kernel-added subnet route for lbr0
	route := netlink.Route{
		Scope:     netlink.SCOPE_LINK,
		Dst:       dev.nodeNetwork.ToIPNet(),
		Src:       dev.gwAddr,
		LinkIndex: ifindex,
	}
	if err := dev.opHandler.RouteDelWithProtocol(&route, syscall.RTPROT_UNSPEC); err != nil && err != syscall.ENOENT {
		log.Errorf("Failed to remove route initial: %v ", err)
	}

	for {
		msgs, err := nlsock.Receive()
		if err != nil {
			log.Errorf("Failed to receive from netlink: %v ", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, msg := range msgs {
			if dev.matchRoute(msg, &route) {
				dev.opHandler.RouteDelWithProtocol(&route, syscall.RTPROT_UNSPEC)
				break
			}
		}
	}
}

func (dev *ovsDevice) ensureDockerBridge() error {
	// Remove any existing lbr0
	lbr0, err := dev.opHandler.LinkByName("lbr0")
	if err == nil {
		dev.opHandler.LinkDel(lbr0)
	}

	// Add lbr0 with clean config
	lbr0 = &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: "lbr0",
		},
	}
	if err := dev.opHandler.LinkAdd(lbr0); err != nil {
		return fmt.Errorf("failed to add lbr0: %v", err)
	}

	// Remove the kernel-added subnet route for lbr0
	// Watch for it and delete it if it shows up later
	nlsock, err := nl.Subscribe(syscall.NETLINK_ROUTE, syscall.RTNLGRP_IPV4_ROUTE)
	if err != nil {
		return fmt.Errorf("failed to subscribe to netlink RTNLGRP_IPV4_ROUTE messages: %v", err)
	}

	go dev.removeLbrSubnetRoute(nlsock, lbr0.Attrs().Index)

	// Add the gateway address to it (for docker IPAM) and bring it up
	lbr0, err = dev.addGatewayAddressToLink("lbr0")
	if err != nil {
		return err
	}

	// Add the link between docker's lbr0 and our OVS bridge
	dev.vsCtl("del-port", dev.bridgeName, "vovsbr")
	link := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:   "vlinuxbr",
			TxQLen: 0,
		},
		PeerName: "vovsbr",
	}

	if err := dev.opHandler.LinkAdd(link); err != nil && err != syscall.EEXIST {
		return fmt.Errorf("failed to add vlinuxbr/vovsbr veth pair: %v", err)
	}

	// One side gets attached to lbr0
	vlinuxbr, err := dev.opHandler.LinkByName("vlinuxbr")
	if err != nil {
		return fmt.Errorf("failed to get vlinuxbr: %v", err)
	}
	if _, ok := vlinuxbr.(*netlink.Veth); !ok {
		return fmt.Errorf("vlinuxbr not a veth")
	}
	master, ok := lbr0.(*netlink.Bridge)
	if !ok {
		return fmt.Errorf("lbr0 not a bridge")
	}
	if err := dev.opHandler.LinkSetMaster(vlinuxbr, master); err != nil {
		return fmt.Errorf("failed to add vlinuxbr to lbr0: %v", err)
	}
	if err := dev.opHandler.LinkSetUp(vlinuxbr); err != nil {
		return fmt.Errorf("failed to bring up vlinuxbr: %v", err)
	}

	// And the other side to our OVS bridge
	vovsbr, err := dev.opHandler.LinkByName(link.PeerName)
	if err != nil {
		return fmt.Errorf("failed to get vovsbr: %v", err)
	}
	if err := dev.opHandler.LinkSetUp(vovsbr); err != nil {
		return fmt.Errorf("failed to bring up vovsbr: %v", err)
	}

	if err := dev.vsCtl("add-port", dev.bridgeName, "vovsbr", "--", "set", "Interface", "vovsbr", "ofport_request=3"); err != nil {
		return fmt.Errorf("failed to attach vovsbr to %s: %v", dev.bridgeName, err)
	}

	return nil
}

func (dev *ovsDevice) ensureTun() error {
	dev.vsCtl("del-port", dev.bridgeName, "tun0")
	if err := dev.vsCtl("add-port", dev.bridgeName, "tun0", "--", "set", "Interface", "tun0", "type=internal", "ofport_request=2"); err != nil {
		return fmt.Errorf("failed to add tun0: %v", err)
	}

	tun0, err := dev.addGatewayAddressToLink("tun0")
	if err != nil {
		return err
	}

	route := netlink.Route{
		Scope:     netlink.SCOPE_LINK,
		Dst:       dev.nodeNetwork.ToIPNet(),
		LinkIndex: tun0.Attrs().Index,
	}

	if err := dev.opHandler.RouteAdd(&route); err != nil && err != syscall.EEXIST {
		return fmt.Errorf("failed to add tun0 network route: %v", err)
	}

	// Set up NAT for containers through tun0
	if err := dev.opHandler.Sysctl("net.ipv4.ip_forward", "1"); err != nil {
		return fmt.Errorf("failed to enable IPv4 forwarding: %v", err)
	}
	if err := dev.opHandler.Sysctl("net.ipv4.conf.tun0.forwarding", "1"); err != nil {
		return fmt.Errorf("failed to enable IPv4 forwarding for tun0: %v", err)
	}

	return nil
}

// Generate the default gateway IP Address for a subnet
func generateDefaultGateway(sna *net.IPNet) net.IP {
	ip := sna.IP.To4()
	return net.IPv4(ip[0], ip[1], ip[2], ip[3]|0x1)
}

func (dev *ovsDevice) nodeSetup(clusterNetwork ip.IP4Net, nodeNetwork ip.IP4Net, servicesNetwork *ip.IP4Net) error {
	dev.clusterNetwork = clusterNetwork
	dev.nodeNetwork = nodeNetwork
	dev.servicesNetwork = *servicesNetwork
	dev.gwAddr = generateDefaultGateway(nodeNetwork.ToIPNet())

	// Finish early if everything is already configured
	// FIXME: need to be finer-grained about this
	if lbr0, err := dev.opHandler.LinkByName("lbr0"); err == nil {
		gwAddrNetlink, _ := netlink.ParseAddr(fmt.Sprintf("%s/%d", dev.gwAddr.String(), dev.nodeNetwork.PrefixLen))
		if addrs, err := dev.opHandler.AddrList(lbr0, syscall.AF_INET); err == nil {
			for _, addr := range addrs {
				if addr.Equal(*gwAddrNetlink) {
					return nil
				}
			}
		}
	}

	if err := dev.ensureDockerBridge(); err != nil {
		return fmt.Errorf("failed docker bridge setup: %v", err)
	}

	if err := dev.ensureTun(); err != nil {
		return fmt.Errorf("failed tun0 setup: %v", err)
	}

	// disable iptables for lbr0
	// for kernel version 3.18+, module br_netfilter needs to be loaded upfront
	// for older ones, br_netfilter may not exist, but is covered by bridge (bridge-utils)
	dev.opHandler.Modprobe("br_netfilter")
	if err := dev.opHandler.Sysctl("net.bridge.bridge-nf-call-iptables", "0"); err != nil {
		return fmt.Errorf("failed bridge-nf-call-iptables setup: %v", err)
	}

	// Clean up any old docker bridge
	docker0, err := dev.opHandler.LinkByName("docker0")
	if err == nil {
		dev.opHandler.LinkDel(docker0)
	}

	// Table 2; incoming from vxlan
	dev.addFlow("table=2, priority=200, ip, nw_dst=%s, actions=output:2", dev.gwAddr.String())
	dev.addFlow("table=2, priority=100, ip, nw_dst=%s, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[], goto_table:6", dev.nodeNetwork.String())

	// Table 4; services; mostly filled in by flannelmt.go
	dev.addFlow("table=4, priority=100, ip, nw_dst=%s, actions=drop", dev.servicesNetwork.String())

	// Table 5; general routing
	dev.addFlow("table=5, priority=200, ip, nw_dst=%s, actions=output:2", dev.gwAddr.String())
	dev.addFlow("table=5, priority=150, ip, nw_dst=%s, actions=goto_table:6", dev.nodeNetwork.String())
	dev.addFlow("table=5, priority=100, ip, nw_dst=%s, actions=goto_table:7", dev.clusterNetwork.String())

	return nil
}

func (dev *ovsDevice) genericSetup() error {
	// Exit early if setup isn't required
	output, err := dev.ofCtl("dump-flows", "")
	if err == nil && strings.Index(string(output), "NXM_NX_TUN_IPV4") >= 0 {
		log.Infof("Skipping generic setup; OVS bridge already initialized")
		return nil
	}

	dev.vsCtl("del-br", dev.bridgeName)
	dev.vsCtl("add-br", dev.bridgeName, "--", "set", "Bridge", dev.bridgeName, "fail-mode=secure")
	dev.vsCtl("set", "bridge", dev.bridgeName, "protocols=OpenFlow13")
	dev.vsCtl("del-port", dev.bridgeName, "vxlan0")
	dev.vsCtl("add-port", dev.bridgeName, "vxlan0", "--", "set", "Interface", "vxlan0", "type=vxlan", "options:remote_ip=\"flow\"", "options:key=\"flow\"", "ofport_request=1")

	link, err := dev.opHandler.LinkByName("br0")
	if err != nil {
		return fmt.Errorf("failed to get br0: %v", err)
	}
	if err := dev.opHandler.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring up br0: %v", err)
	}

	// Table 0; learn MAC addresses and continue with table 1
	dev.addFlow("table=0, actions=learn(table=8, priority=200, hard_timeout=900, NXM_OF_ETH_DST[]=NXM_OF_ETH_SRC[], load:NXM_NX_TUN_IPV4_SRC[]->NXM_NX_TUN_IPV4_DST[], output:NXM_OF_IN_PORT[]), goto_table:1")

	// Table 1; initial dispatch
	dev.addFlow("table=1, arp, actions=goto_table:8")
	dev.addFlow("table=1, in_port=1, actions=goto_table:2") // vxlan0
	dev.addFlow("table=1, in_port=2, actions=goto_table:5") // tun0
	dev.addFlow("table=1, in_port=3, actions=goto_table:5") // vovsbr
	dev.addFlow("table=1, actions=goto_table:3")            // container

	// Table 2; incoming from vxlan
	dev.addFlow("table=2, arp, actions=goto_table:8")
	dev.addFlow("table=2, tun_id=0, actions=goto_table:5")

	// Table 3; incoming from container; filled in by openshift-ovs-flannelmt

	// Table 4; services; mostly filled in by flannelmt.go
	dev.addFlow("table=4, priority=0, actions=goto_table:5")

	// Table 5; general routing
	dev.addFlow("table=5, priority=0, ip, actions=output:2")

	// Table 6; to local container; mostly filled in by openshift-ovs-flannelmt
	dev.addFlow("table=6, priority=200, ip, reg0=0, actions=goto_table:8")

	// Table 7; to remote container; filled in by flannelmt.go

	// Table 8; MAC dispatch / ARP, filled in by Table 0's learn() rule
	// and with per-node vxlan ARP rules by multitenant.go
	dev.addFlow("table=8, priority=0, arp, actions=flood")

	return nil
}

func (dev *ovsDevice) Destroy() {
	dev.vsCtl("del-br", dev.bridgeName)
}

func generateCookie(ip string) string {
	ipa, _, err := net.ParseCIDR(ip)
	if err != nil {
		ipa = net.ParseIP(ip)
	}
	return hex.EncodeToString(ipa.To4())
}

func (dev *ovsDevice) AddRemoteSubnet(lease *net.IPNet, vtep net.IP) error {
	cookie := generateCookie(vtep.String())

	if err := dev.addFlow("table=7,cookie=0x%s,priority=100,ip,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, lease.String(), vtep.String()); err != nil {
		return err
	}

	if err := dev.addFlow("table=8,cookie=0x%s,priority=100,arp,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, lease.String(), vtep.String()); err != nil {
		return err
	}

	return nil
}

func (dev *ovsDevice) RemoveRemoteSubnet(lease *net.IPNet, vtep net.IP) error {
	cookie := generateCookie(vtep.String())

	dev.delFlows("table=7,cookie=0x%s/0xffffffff", cookie)
	dev.delFlows("table=8,cookie=0x%s/0xffffffff", cookie)
	return nil
}
