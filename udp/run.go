package udp

import (
	"encoding/json"
	"net"
	"time"

	"github.com/coreos-inc/rudder/Godeps/_workspace/src/github.com/docker/libcontainer/netlink"
	log "github.com/coreos-inc/rudder/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos-inc/rudder/backend"
	"github.com/coreos-inc/rudder/pkg"
	"github.com/coreos-inc/rudder/subnet"
)

const (
	encapOverhead = 28 // 20 bytes IP hdr + 8 bytes UDP hdr
)

func configureIface(ifname string, ipn pkg.IP4Net, mtu int) error {
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
		log.Errorf("Failed to set MTU for %s: ", ifname, err)
		return err
	}

	err = netlink.NetworkLinkUp(iface)
	if err != nil {
		log.Errorf("Failed set interface %s to UP state: %s", ifname, err)
		return err
	}

	return nil
}

func acquireLease(sm *subnet.SubnetManager, pubIP net.IP) (pkg.IP4Net, error) {
	attrs := subnet.BaseAttrs{
		PublicIP: pkg.FromIP(pubIP),
	}
	data, err := json.Marshal(&attrs)
	if err != nil {
		return pkg.IP4Net{}, err
	}

	var sn pkg.IP4Net
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

func Run(sm *subnet.SubnetManager, iface *net.Interface, ip net.IP, port int, fast bool, ready backend.ReadyFunc) {
	sn, err := acquireLease(sm, ip)
	if err != nil {
		log.Error("Failed to acquire lease: ", err)
		return
	}

	tun, tunName, err := pkg.OpenTun("rudder%d")
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
	ipn := pkg.IP4Net{
		IP:        sn.IP,
		PrefixLen: sm.GetConfig().Network.PrefixLen,
	}

	// TUN MTU will be smaller b/c of encap (IP+UDP hdrs)
	var mtu int
	if iface.MTU > 0 {
		mtu = iface.MTU - encapOverhead
	} else {
		log.Errorf("Failed to determine MTU for %s interface", ip)
		return
	}

	err = configureIface(tunName, ipn, mtu)
	if err != nil {
		return
	}

	// all initialized and ready for business
	log.Info("UDP encapsulation initialized")
	ready(sn, mtu)

	if fast {
		fastProxy(sm, tun, conn, ipn.IP, uint(mtu), port)
	} else {
		proxy(sm, tun, conn, uint(mtu), port)
	}
}
