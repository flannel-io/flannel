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

package ovs

import (
	"bytes"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unicode"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/vishvananda/netlink"

	"github.com/coreos/flannel/pkg/ip"
)

//*****************************************************************************

type ovsPort struct {
	name  string
	ptype string
	port  int
}

type testLink struct {
	link   netlink.Link
	addrs  []netlink.Addr
	routes []netlink.Route
}

type testOpHandler struct {
	bridge   string
	proto    string
	nextLink int
	links    map[string]*testLink
	ovsPorts map[string]*ovsPort
	ovsFlows []string
	sysctls  map[string]string
}

type ovsFunc func(h *testOpHandler, args []string) ([]byte, error)

var vsCtlMap = map[string]ovsFunc{
	"add-br":   vsAddBr,
	"del-br":   vsDelBr,
	"set":      vsSet,
	"add-port": vsAddPort,
	"del-port": vsDelPort,
}

var tt *testing.T

func vsAddBr(h *testOpHandler, args []string) ([]byte, error) {
	if h.bridge != "" || len(h.ovsPorts) > 0 || len(h.ovsFlows) > 0 {
		return nil, fmt.Errorf("OVS bridge already exists")
	}

	h.bridge = args[0]
	h.links = make(map[string]*testLink)
	h.ovsPorts = make(map[string]*ovsPort)
	h.sysctls = make(map[string]string)
	h.nextLink = 1

	// Add the corresponding kernel link
	link := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{
			Name: h.bridge,
		},
	}
	err := h.LinkAdd(link)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func vsDelBr(h *testOpHandler, args []string) ([]byte, error) {
	// Delete kernel link
	if _, ok := h.links[h.bridge]; ok {
		delete(h.links, h.bridge)
	}

	for k := range h.ovsPorts {
		delete(h.ovsPorts, k)
	}
	h.ovsFlows = nil
	h.sysctls = nil
	h.bridge = ""
	return nil, nil
}

func vsSet(h *testOpHandler, args []string) ([]byte, error) {
	for _, a := range args {
		if strings.HasPrefix(a, "protocols=") {
			h.proto = a[10:]
		}
	}
	return nil, nil
}

func (h *testOpHandler) ovsPortExists(port int) bool {
	for _, p := range h.ovsPorts {
		if p.port == port {
			return true
		}
	}
	return false
}

func (h *testOpHandler) nextOvsPort() (int, error) {
	// OVS always starts added ports at 1 because 0 is the LOCAL port (the bridge itself)
	for i := 1; i < 1000; i++ {
		if !h.ovsPortExists(i) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("Exhausted available OVS port numbers")
}

func vsAddPort(h *testOpHandler, args []string) ([]byte, error) {
	name := args[1]
	if _, ok := h.ovsPorts[name]; ok {
		return nil, fmt.Errorf("Bridge port %s already exists", name)
	}
	port := &ovsPort{
		name: name,
		port: -1,
	}
	addLink := false
	for _, a := range args {
		if strings.HasPrefix(a, "type=") {
			port.ptype = a[5:]

			// only type=internal ports get created as a netdev
			if port.ptype == "internal" {
				addLink = true
			}
		} else if strings.HasPrefix(a, "ofport_request=1") {
			pnum, err := strconv.ParseInt(a[15:], 10, 32)
			if err == nil {
				// ovs-vsctl assigns a new port number if the
				// requested one is taken
				if !h.ovsPortExists(int(pnum)) {
					port.port = int(pnum)
				}
			}
		}
	}

	if port.port < 0 {
		port.port, _ = h.nextOvsPort()
	}

	h.ovsPorts[name] = port

	if addLink {
		// Add the corresponding kernel link
		link := &netlink.Device{
			LinkAttrs: netlink.LinkAttrs{
				Name: name,
			},
		}
		err := h.LinkAdd(link)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func vsDelPort(h *testOpHandler, args []string) ([]byte, error) {
	name := args[1]
	if _, ok := h.ovsPorts[name]; !ok {
		return nil, fmt.Errorf("Bridge port %s does not exist", name)
	}
	delete(h.ovsPorts, name)

	// Delete kernel link
	if _, ok := h.links[name]; ok {
		delete(h.links, name)
	}

	return nil, nil
}

func (h *testOpHandler) vsCtl(args ...string) ([]byte, error) {
	funcName := args[0]
	vsfunc, ok := vsCtlMap[funcName]
	if !ok {
		tt.Errorf("Unknown ovs-vsctl command %s", funcName)
	}
	bridgeIndex := 1
	if funcName == "set" {
		bridgeIndex = 2
	}
	if h.bridge != "" && args[bridgeIndex] != h.bridge {
		return nil, fmt.Errorf("Unexpected switch name %s", args[bridgeIndex])
	}

	return vsfunc(h, args[bridgeIndex:])
}

var ofCtlMap = map[string]ovsFunc{
	"add-flow":   ofAddFlow,
	"del-flows":  ofDelFlows,
	"dump-ports": ofDumpPorts,
	"dump-flows": ofDumpFlows,
}

func canonicalizeFlow(flow string) string {
	var buffer bytes.Buffer
	for _, c := range flow {
		if !unicode.IsSpace(c) {
			buffer.WriteString(fmt.Sprintf("%c", c))
		}
	}
	return buffer.String()
}

func ofAddFlow(h *testOpHandler, args []string) ([]byte, error) {
	// Strip internal spaces
	if len(args) != 1 {
		return nil, fmt.Errorf("Expected only one flow argument (got %d)", len(args))
	}

	h.ovsFlows = append(h.ovsFlows, canonicalizeFlow(args[0]))

	return nil, nil
}

// Returns table # and cookie of the flow
func parseFlow(flow string) (string, string, error) {
	var table string
	var cookie string

	for _, p := range strings.Split(flow, ",") {
		if strings.HasPrefix(p, "table=") {
			table = p[6:]
		} else if strings.HasPrefix(p, "cookie=0x") {
			slash := strings.Index(p, "/")
			if slash >= 0 && slash <= 9 {
				return "", "", fmt.Errorf("Invalid OVS flow to delete; cookie format wrong: %s", flow)
			} else if slash > 9 {
				cookie = p[9:slash]
			} else {
				cookie = p[9:]
			}
		}
		if table != "" && cookie != "" {
			break
		}
	}
	return table, cookie, nil
}

func ofDelFlows(h *testOpHandler, args []string) ([]byte, error) {
	table, cookie, err := parseFlow(args[0])
	if err != nil {
		return nil, err
	}

loop:
	for idx, f := range h.ovsFlows {
		matchedTable := true
		matchedCookie := true
		ct, cc, _ := parseFlow(f)
		if table != "" && table != ct {
			matchedTable = false
		}
		if cookie != "" && cookie != cc {
			matchedCookie = false
		}
		if matchedTable && matchedCookie {
			h.ovsFlows = append(h.ovsFlows[:idx], h.ovsFlows[idx+1:]...)
			goto loop
		}
	}

	return nil, nil
}

type ovsPortSort []*ovsPort

func (s ovsPortSort) Len() int {
	return len(s)
}

func (s ovsPortSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ovsPortSort) Less(i, j int) bool {
	return s[i].port < s[j].port
}

func ofDumpPorts(h *testOpHandler, args []string) ([]byte, error) {
	var buffer bytes.Buffer

	dumpPorts := make(ovsPortSort, 0, 10)
	if len(args) == 1 {
		port, ok := h.ovsPorts[args[0]]
		if !ok {
			buffer.WriteString(fmt.Sprintf("ovs-ofctl: br0: couldn't find port `%s'", args[0]))
			return buffer.Bytes(), nil
		}
		dumpPorts = append(dumpPorts, port)
	} else {
		for _, port := range h.ovsPorts {
			if len(args) == 1 && port.name == args[0] {
				break
			}
		}
		sort.Sort(ovsPortSort(dumpPorts))
	}

	buffer.WriteString(fmt.Sprintf("OFPST_PORT reply (xid=0x1): %s ports\n", len(h.ovsPorts)))
	for _, port := range dumpPorts {
		buffer.WriteString(fmt.Sprintf("port %d: rx pkts=33, bytes=5785, drop=0, errs=0, frame=0, over=0, crc=0\n", port.port))
		buffer.WriteString("         tx pkts=730, bytes=56061, drop=0, errs=0, coll=0\n")
	}

	return buffer.Bytes(), nil
}

func ofDumpFlows(h *testOpHandler, args []string) ([]byte, error) {
	var buffer bytes.Buffer

	buffer.WriteString("OFPST_FLOW reply (OF1.3) (xid=0x2):\n")
	for _, flow := range h.ovsFlows {
		buffer.WriteString(fmt.Sprintf(" duration=5.690s, %s\n", flow))
	}

	return buffer.Bytes(), nil
}

func (h *testOpHandler) ofCtl(args ...string) ([]byte, error) {
	funcName := args[2]
	offunc, ok := ofCtlMap[funcName]
	if !ok {
		tt.Fatalf("Unknown ovs-ofctl command %s", funcName)
	}
	if args[0] != "-O" {
		tt.Fatalf("Expected OpenFlow protocol flag -O at position 1")
	}
	bridgeName := args[3]
	if h.bridge == "" {
		var buffer bytes.Buffer

		buffer.WriteString(fmt.Sprintf("ovs-ofctl: %s is not a bridge or a socket", bridgeName))
		return buffer.Bytes(), nil
	} else {
		if args[1] != h.proto {
			tt.Fatalf("Unexpected OpenFlow protocol %s (have %s)", args[1], h.proto)
		}
		if bridgeName != h.bridge {
			tt.Fatalf("Unexpected bridge name %s", bridgeName)
		}
	}

	return offunc(h, args[4:])
}

func (h *testOpHandler) OvsExec(cmd string, args ...string) ([]byte, error) {
	switch cmd {
	case "ovs-vsctl":
		return h.vsCtl(args...)
	case "ovs-ofctl":
		return h.ofCtl(args...)
	}
	tt.Fatalf("Unknown OVS command %s", cmd)
	return nil, fmt.Errorf("Unknown OVS command %s", cmd)
}

func (h *testOpHandler) Modprobe(module string) ([]byte, error) {
	return nil, nil
}

func (h *testOpHandler) LinkAdd(link netlink.Link) error {
	base := link.Attrs()
	if _, ok := h.links[base.Name]; ok {
		return syscall.Errno(syscall.EEXIST)
	}

	var peerName string
	if veth, ok := link.(*netlink.Veth); ok {
		if _, ok := h.links[veth.PeerName]; ok {
			return syscall.Errno(syscall.EEXIST)
		}
		peerName = veth.PeerName
	}

	base.Index = h.nextLink
	h.nextLink += 1

	h.links[base.Name] = &testLink{
		link: link,
	}

	if peerName != "" {
		// Add the peer
		peer := &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{
				Name:   peerName,
				TxQLen: base.TxQLen,
				Index:  h.nextLink,
			},
			PeerName: base.Name,
		}
		h.nextLink += 1

		h.links[peerName] = &testLink{
			link: peer,
		}
	}

	return nil
}

func (h *testOpHandler) LinkDel(link netlink.Link) error {
	base := link.Attrs()
	if _, ok := h.links[base.Name]; ok {
		return syscall.Errno(syscall.EEXIST)
	}

	var peerName string
	if veth, ok := link.(*netlink.Veth); ok {
		if _, ok := h.links[veth.PeerName]; ok {
			return syscall.Errno(syscall.EEXIST)
		}
		peerName = veth.PeerName
	}

	delete(h.links, base.Name)
	if peerName != "" {
		delete(h.links, peerName)
	}

	return nil
}

func (h *testOpHandler) LinkByName(name string) (netlink.Link, error) {
	link, ok := h.links[name]
	if !ok {
		return nil, syscall.Errno(syscall.ENOENT)
	}
	return link.link, nil
}

func (h *testOpHandler) LinkByIndex(index int) (netlink.Link, error) {
	for _, l := range h.links {
		if index == l.link.Attrs().Index {
			return l.link, nil
		}
	}
	return nil, syscall.Errno(syscall.ENOENT)
}

func (h *testOpHandler) LinkSetUp(link netlink.Link) error {
	base := link.Attrs()
	found, ok := h.links[base.Name]
	if !ok {
		return syscall.Errno(syscall.ENOENT)
	}
	found.link.Attrs().Flags |= net.FlagUp
	return nil
}

func (h *testOpHandler) LinkSetMaster(link netlink.Link, master *netlink.Bridge) error {
	base := link.Attrs()
	found, ok := h.links[base.Name]
	if !ok {
		return syscall.Errno(syscall.ENOENT)
	}
	found.link.Attrs().MasterIndex = master.Index
	return nil
}

func (h *testOpHandler) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	base := link.Attrs()
	found, ok := h.links[base.Name]
	if !ok {
		return syscall.Errno(syscall.ENOENT)
	}

	for _, a := range found.addrs {
		if a.Equal(*addr) {
			return syscall.Errno(syscall.EEXIST)
		}
	}
	found.addrs = append(found.addrs, *addr)

	// Add the subnet route for this address too
	if i, n, err := net.ParseCIDR(strings.TrimSpace(addr.String())); err == nil {
		route := netlink.Route{
			Scope:     netlink.SCOPE_LINK,
			Dst:       n,
			Src:       i,
			LinkIndex: base.Index,
		}
		h.RouteAdd(&route)
	}

	return nil
}

func (h *testOpHandler) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	base := link.Attrs()
	found, ok := h.links[base.Name]
	if !ok {
		return nil, syscall.Errno(syscall.ENOENT)
	}

	return found.addrs, nil
}

func (h *testOpHandler) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	base := link.Attrs()
	found, ok := h.links[base.Name]
	if !ok {
		return nil, syscall.Errno(syscall.ENOENT)
	}

	return found.routes, nil
}

func (h *testOpHandler) findByIndex(index int) (*testLink, error) {
	for _, l := range h.links {
		if l.link.Attrs().Index == index {
			return l, nil
		}
	}
	return nil, syscall.Errno(syscall.ENOENT)
}

func (h *testOpHandler) RouteAdd(route *netlink.Route) error {
	found, err := h.findByIndex(route.LinkIndex)
	if err != nil {
		return err
	}

	for _, r := range found.routes {
		if reflect.DeepEqual(r, route) {
			return syscall.Errno(syscall.EEXIST)
		}
	}
	found.routes = append(found.routes, *route)
	return nil
}

func (h *testOpHandler) RouteDel(route *netlink.Route) error {
	found, err := h.findByIndex(route.LinkIndex)
	if err != nil {
		return err
	}

	for idx, r := range found.routes {
		if reflect.DeepEqual(r, route) {
			// Delete route from the list
			found.routes = append(found.routes[:idx], found.routes[idx+1:]...)
			return nil
		}
	}
	return syscall.Errno(syscall.ENOENT)
}

func (h *testOpHandler) RouteDelWithProtocol(route *netlink.Route, protocol uint8) error {
	return h.RouteDel(route)
}

func (h *testOpHandler) Sysctl(item, value string) error {
	h.sysctls[item] = value
	return nil
}

func (h *testOpHandler) MatchSwitchConfig(name string, ports []ovsPort) error {
	if name != h.bridge {
		return fmt.Errorf("Bad bridge name %s (expected %s)", h.bridge, name)
	}

	if len(ports) != len(h.ovsPorts) {
		return fmt.Errorf("Expected ports %d doesn't match found ports %d", len(ports), len(h.ovsPorts))
	}

	for _, expected := range ports {
		found, ok := h.ovsPorts[expected.name]
		if !ok {
			return fmt.Errorf("Failed to find expected port %s", expected.name)
		}
		if found.name != expected.name {
			return fmt.Errorf("Bad port name %s (expected %s)", found.name, expected.name)
		}
		if found.ptype != expected.ptype {
			return fmt.Errorf("Bad port type %s (expected %s)", found.ptype, expected.ptype)
		}
		if found.port != expected.port {
			return fmt.Errorf("Bad port number %s (expected %s)", found.port, expected.port)
		}
	}
	return nil
}

func (h *testOpHandler) MatchFlows(flows []string) error {
	tmp := make(map[string]bool)
	for _, k := range h.ovsFlows {
		tmp[k] = false
	}

	for _, expected := range flows {
		expected := canonicalizeFlow(expected)
		found := false
		for _, candidate := range h.ovsFlows {
			if expected == candidate {
				tmp[candidate] = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Unmatched expected flow '%s'", expected)
		}
	}

	for k := range tmp {
		if tmp[k] != true {
			return fmt.Errorf("Unmatched flow in OVS table '%s'", k)
		}
	}

	if len(flows) != len(h.ovsFlows) {
		return fmt.Errorf("Expected flows %d doesn't match found flows %d", len(flows), len(h.ovsFlows))
	}

	return nil
}

func newTestOpHandler() *testOpHandler {
	return &testOpHandler{
		ovsPorts: make(map[string]*ovsPort),
		links:    make(map[string]*testLink),
		sysctls:  make(map[string]string),
		nextLink: 1,
	}
}

//*****************************************************************************

type expectedLink struct {
	ip      string
	ovsType string
}

func IPNetFromString(cidr string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		tt.Fatalf("Failed to parse IP address %s: %v", cidr, err)
	}
	return ipnet
}

func TestOVSBridge(t *testing.T) {
	tt = t
	opHandler := newTestOpHandler()
	dev, err := newOVSDeviceWithHandler(opHandler)
	if err != nil {
		t.Fatalf("Failed to create OVS bridge device: %s", err)
	}

	// Only one kernel link: br0
	if len(opHandler.links) != 1 {
		t.Fatalf("Found unexpected number of kernel links %d (expected 1)", len(opHandler.links))
	}

	expectedPorts := []ovsPort{
		{"vxlan0", "vxlan", 1},
	}
	if err := opHandler.MatchSwitchConfig("br0", expectedPorts); err != nil {
		t.Fatalf("Failed to match vswitch config: %s", err)
	}

	expectedFlows := []string{
		"table=0, actions=learn(table=8, priority=200, hard_timeout=900, NXM_OF_ETH_DST[]=NXM_OF_ETH_SRC[], load:NXM_NX_TUN_IPV4_SRC[]->NXM_NX_TUN_IPV4_DST[], output:NXM_OF_IN_PORT[]), goto_table:1",
		"table=1, arp, actions=goto_table:8",
		"table=1, in_port=1, actions=goto_table:2",
		"table=1, in_port=2, actions=goto_table:5",
		"table=1, in_port=3, actions=goto_table:5",
		"table=1, actions=goto_table:3",
		"table=2, arp, actions=goto_table:8",
		"table=2, tun_id=0, actions=goto_table:5",
		"table=4, priority=0, actions=goto_table:5",
		"table=5, priority=0, ip, actions=output:2",
		"table=6, priority=200, ip, reg0=0, actions=goto_table:8",
		"table=8, priority=0, arp, actions=flood",
	}
	if err := opHandler.MatchFlows(expectedFlows); err != nil {
		t.Fatalf("Failed to match flows: %s", err)
	}

	// Do final node setup
	clusterNetwork := ip.FromIPNet(IPNetFromString("10.1.0.0/16"))
	nodeNetwork := ip.FromIPNet(IPNetFromString("10.1.55.0/24"))
	servicesNetwork := ip.FromIPNet(IPNetFromString("172.16.1.0/24"))
	if err := dev.nodeSetup(clusterNetwork, nodeNetwork, &servicesNetwork); err != nil {
		t.Fatalf("Failed to add network to device: %s", err)
	}

	gwAddr := "10.1.55.1"
	gwCidr := fmt.Sprintf("%s/24", gwAddr)

	// Should now have a bunch of links now: br0, lbr0, vovsbr, vlinuxbr, tun0
	expectedLinks := make(map[string]expectedLink)
	expectedLinks["br0"] = expectedLink{"", ""}
	expectedLinks["lbr0"] = expectedLink{gwCidr, ""}
	expectedLinks["vovsbr"] = expectedLink{"", ""}
	expectedLinks["vlinuxbr"] = expectedLink{"", ""}
	expectedLinks["tun0"] = expectedLink{gwCidr, "internal"}

	if len(opHandler.links) != len(expectedLinks) {
		t.Fatalf("Found unexpected number of kernel links %d (expected %d)", len(opHandler.links), len(expectedLinks))
	}

	for name, el := range expectedLinks {
		l, ok := opHandler.links[name]
		if !ok {
			t.Fatalf("Failed to find kernel link %s", name)
		}
		if el.ip != "" {
			if len(l.addrs) != 1 {
				t.Fatalf("Unexpected number of IP address on %s: %d", name, len(l.addrs))
			}
			// netlink's String() adds a space at the end...
			if strings.TrimSpace(l.addrs[0].String()) != el.ip {
				t.Fatalf("Mismatched IP address on %s: %s (expected %s)", name, l.addrs[0].String(), el.ip)
			}
		} else if len(l.addrs) != 0 {
			t.Fatalf("Unexpected IP address on %s: %s", name, l.addrs[0].String())
		}

		if l.link.Attrs().Flags&net.FlagUp == 0 {
			t.Fatalf("Link %s was not UP", name)
		}

		if el.ovsType != "" {
			o, ok := opHandler.ovsPorts[name]
			if !ok {
				t.Fatalf("Failed to find OVS link %s", name)
			}
			if o.ptype != el.ovsType {
				t.Fatalf("Unexpected OVS link %s type %s (expected %s)", name, o.ptype, el.ovsType)
			}
		}
	}

	// Sysctls
	expectedSysctls := make(map[string]string)
	expectedSysctls["net.bridge.bridge-nf-call-iptables"] = "0"
	expectedSysctls["net.ipv4.ip_forward"] = "1"
	expectedSysctls["net.ipv4.conf.tun0.forwarding"] = "1"

	if len(opHandler.sysctls) != len(expectedSysctls) {
		t.Fatalf("Found unexpected number of sysctls %d (expected %d)", len(opHandler.sysctls), len(expectedSysctls))
	}

	for item, value := range expectedSysctls {
		sc, ok := opHandler.sysctls[item]
		if !ok {
			t.Fatalf("Failed to find sysctl %s", item)
		}
		if sc != value {
			t.Fatalf("Unexpected sysctl %s value %s (expected %s)", item, sc, value)
		}
	}

	nodeFlows := []string{
		fmt.Sprintf("table=2, priority=200, ip, nw_dst=%s, actions=output:2", gwAddr),
		fmt.Sprintf("table=2, priority=100, ip, nw_dst=%s, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[], goto_table:6", nodeNetwork.String()),
		fmt.Sprintf("table=4, priority=100, ip, nw_dst=%s, actions=drop", servicesNetwork.String()),
		fmt.Sprintf("table=5, priority=200, ip, nw_dst=%s, actions=output:2", gwAddr),
		fmt.Sprintf("table=5, priority=150, ip, nw_dst=%s, actions=goto_table:6", nodeNetwork.String()),
		fmt.Sprintf("table=5, priority=100, ip, nw_dst=%s, actions=goto_table:7", clusterNetwork.String()),
	}
	for _, f := range expectedFlows {
		nodeFlows = append(nodeFlows, f)
	}

	if err := opHandler.MatchFlows(nodeFlows); err != nil {
		t.Fatalf("Failed to match node flows: %s", err)
	}

}

func TestSkipOVSSetup(t *testing.T) {
	tt = t
	opHandler := newTestOpHandler()

	// Add one bogus flow and make sure the bridge doesn't add any more
	_, err := opHandler.OvsExec("ovs-vsctl", "add-br", "br0", "--", "set", "Bridge", "br0")
	if err != nil {
		tt.Fatalf("Failed to add OVS bridge: %v", err)
	}
	_, err = opHandler.OvsExec("ovs-vsctl", "set", "bridge", "br0", "protocols=OpenFlow13")
	if err != nil {
		tt.Fatalf("Failed to set OVS bridge protocol: %v", err)
	}

	flow := "table=0, actions=learn(table=8, priority=200, hard_timeout=900, NXM_OF_ETH_DST[]=NXM_OF_ETH_SRC[], load:NXM_NX_TUN_IPV4_SRC[]->NXM_NX_TUN_IPV4_DST[], output:NXM_OF_IN_PORT[]), goto_table:1"
	opHandler.ofCtl("-O", "OpenFlow13", "add-flow", "br0", flow)

	// Let the device try to configure the bridge
	_, err = newOVSDeviceWithHandler(opHandler)
	if err != nil {
		t.Fatalf("Failed to create OVS bridge device: %s", err)
	}

	if len(opHandler.ovsFlows) != 1 {
		t.Fatalf("OVS bridge flows unexpectedly configured")
	}
}

func TestSkipNodeSetup(t *testing.T) {
	tt = t
	opHandler := newTestOpHandler()
	dev, err := newOVSDeviceWithHandler(opHandler)
	if err != nil {
		t.Fatalf("Failed to create OVS bridge device: %s", err)
	}

	clusterNetwork := ip.FromIPNet(IPNetFromString("10.1.0.0/16"))
	nodeNetwork := ip.FromIPNet(IPNetFromString("10.1.55.0/24"))
	servicesNetwork := ip.FromIPNet(IPNetFromString("172.16.1.0/24"))

	// Add lbr0 and the gateway address and make sure no other interfaces get created
	tmp := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: "lbr0",
		},
	}
	if err := opHandler.LinkAdd(tmp); err != nil {
		t.Fatalf("failed to add lbr0: %v", err)
	}

	lbr0, ok := opHandler.links["lbr0"]
	if !ok {
		t.Fatalf("lbr0 not added")
	}

	addr, err := netlink.ParseAddr("10.1.55.1/24")
	err = opHandler.AddrAdd(lbr0.link, addr)
	if err != nil {
		t.Fatalf("failed to parse gateway address %s: %v", addr.String(), err)
	}

	// Let the device do node setup
	if err := dev.nodeSetup(clusterNetwork, nodeNetwork, &servicesNetwork); err != nil {
		t.Fatalf("Failed to add network to device: %s", err)
	}

	// Ensure we only have br0 and lbr0
	if len(opHandler.links) != 2 {
		t.Fatalf("Unexpected number of links %d (expected 2)", len(opHandler.links))
	}

	if _, ok := opHandler.links["br0"]; !ok {
		t.Fatalf("Couldn't find br0")
	}

	if _, ok := opHandler.links["lbr0"]; !ok {
		t.Fatalf("Couldn't find lbr0")
	}
}
