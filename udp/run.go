package udp

import (
	"encoding/json"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/docker/libcontainer/netlink"
	log "github.com/coreos/rudder/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/rudder/backend"
	"github.com/coreos/rudder/pkg/ip"
	"github.com/coreos/rudder/subnet"
)

const (
	encapOverhead = 28 // 20 bytes IP hdr + 8 bytes UDP hdr
)

func configureIface(ifname string, ipn ip.IP4Net, mtu int) error {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		log.Error("Failed to lookup interface ", ifname)
		return err
	}

	n := ipn.ToIPNet()
	err = netlink.NetworkLinkAddIp(iface, n.IP, n)
	if err != nil {
		log.Errorf("Failed to add IP address %s to %s: %s", n.IP, ifname, err)
		return err
	}

	err = netlink.NetworkSetMTU(iface, mtu)
	if err != nil {
		log.Errorf("Failed to set MTU for %s: %v", ifname, err)
		return err
	}

	err = netlink.NetworkLinkUp(iface)
	if err != nil {
		log.Errorf("Failed to set interface %s to UP state: %s", ifname, err)
		return err
	}

	// explicitly add a route since there might be a route for a subnet already
	// installed by Docker and then it won't get auto added
	err = netlink.AddRoute(ipn.Network().String(), "", "", ifname)
	if err != nil && err != syscall.EEXIST {
		log.Errorf("Failed to add route (%s -> %s): %v", ipn.Network().String(), ifname, err)
		return err
	}

	return nil
}

func setupIpMasq(ipn ip.IP4Net, iface string) error {
	ipt, err := ip.NewIPTables()
	if err != nil {
		log.Error("Failed to setup IP Masquerade. iptables was not found")
		return err
	}

	err = ipt.ClearChain("nat", "RUDDER")
	if err != nil {
		log.Error("Failed to create/clear RUDDER chain in NAT table: ", err)
		return err
	}

	rules := [][]string{
		// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
		[]string{"RUDDER", "-d", ipn.String(), "-j", "ACCEPT"},
		// This rule makes sure we don't NAT multicast traffic within overlay network
		[]string{"RUDDER", "-d", "224.0.0.0/4", "-j", "ACCEPT"},
		// This rule will NAT everything originating from our overlay network and
		[]string{"RUDDER", "!", "-o", iface, "-j", "MASQUERADE"},
		// This rule will take everything coming from overlay and sent it to RUDDER chain
		[]string{"POSTROUTING", "-s", ipn.String(), "-j", "RUDDER"},
	}

	for _, args := range rules {
		log.Info("Adding iptables rule: ", strings.Join(args, " "))

		err = ipt.AppendUnique("nat", args...)
		if err != nil {
			log.Error("Failed to insert IP masquerade rule: ", err)
			return err
		}
	}

	return nil
}

func acquireLease(sm *subnet.SubnetManager, pubIP net.IP) (ip.IP4Net, error) {
	attrs := subnet.BaseAttrs{
		PublicIP: ip.FromIP(pubIP),
	}
	data, err := json.Marshal(&attrs)
	if err != nil {
		return ip.IP4Net{}, err
	}

	var sn ip.IP4Net
	for {
		sn, err = sm.AcquireLease(attrs.PublicIP, string(data))
		if err == nil {
			log.Info("Subnet lease acquired: ", sn)
			break
		}
		log.Error("Failed to acquire subnet: ", err)
		time.Sleep(time.Second)
	}

	return sn, nil
}

func Run(sm *subnet.SubnetManager, tepIface *net.Interface, tepIP net.IP, port int, ipMasq bool, ready backend.ReadyFunc) {
	sn, err := acquireLease(sm, tepIP)
	if err != nil {
		log.Error("Failed to acquire lease: ", err)
		return
	}

	tun, tunName, err := ip.OpenTun("rudder%d")
	if err != nil {
		log.Error("Failed to open TUN device: ", err)
		return
	}

	localAddr := net.UDPAddr{
		Port: port,
	}

	conn, err := net.ListenUDP("udp4", &localAddr)
	if err != nil {
		log.Error("Failed to start listening on UDP socket: ", err)
		return
	}

	// Interface's subnet is that of the whole overlay network (e.g. /16)
	// and not that of the individual host (e.g. /24)
	tunNet := ip.IP4Net{
		IP:        sn.IP,
		PrefixLen: sm.GetConfig().Network.PrefixLen,
	}

	// TUN MTU will be smaller b/c of encap (IP+UDP hdrs)
	var mtu int
	if tepIface.MTU > 0 {
		mtu = tepIface.MTU - encapOverhead
	} else {
		log.Errorf("Failed to determine MTU for %s interface", tepIP)
		return
	}

	err = configureIface(tunName, tunNet, mtu)
	if err != nil {
		return
	}

	if ipMasq {
		err = setupIpMasq(tunNet.Network(), tunName)
		if err != nil {
			return
		}
	}

	// all initialized and ready for business
	log.Info("UDP encapsulation initialized")
	ready(sn, mtu)

	fastProxy(sm, tun, conn, tunNet.IP, uint(mtu), port)
}
