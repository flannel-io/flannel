package udp

import (
	"os"
	"net"
	"time"
	"encoding/json"

	"github.com/coreos-inc/kolach/Godeps/_workspace/src/github.com/docker/libcontainer/netlink"
	log "github.com/coreos-inc/kolach/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos-inc/kolach/pkg"
	"github.com/coreos-inc/kolach/subnet"
	"github.com/coreos-inc/kolach/backend"
)

const (
	encapOverhead = 28 // 20 bytes IP hdr + 8 bytes UDP hdr
	defaultMTU    = 1500 - encapOverhead
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

func proxyTunToUdp(r *Router, tun *os.File, conn *net.UDPConn) {
	pkt := make([]byte, 1600)
	for {
		nbytes, err := tun.Read(pkt)
		if err != nil {
			log.V(1).Info("Error reading from TUN device: ", err)
		} else {
			r.routePacket(pkt[:nbytes], conn)
		}
	}
}

func proxyUdpToTun(conn *net.UDPConn, tun *os.File) {
	pkt := make([]byte, 1600)
	for {
		nrecv, err := conn.Read(pkt)
		if err != nil {
			log.V(1).Info("Error reading from socket: ", err)
		} else {
			nsent, err := tun.Write(pkt[:nrecv])
			switch {
			case err != nil:
				log.V(1).Info("Error writing to TUN device: ", err)
			case nsent != nrecv:
				log.V(1).Infof("Was only able to write %d out of %d bytes to TUN device: ", nsent, nrecv)
			}
		}
	}
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

func monitorEvents(sm *subnet.SubnetManager, rtr *Router) {
	evts := make(chan subnet.EventBatch)
	sm.Start(evts)

	for evtBatch := range evts {
		for _, evt := range evtBatch {
			if evt.Type == subnet.SubnetAdded {
				log.Info("Subnet added: ", evt.Lease.Network)
				var attrs subnet.BaseAttrs
				if err := json.Unmarshal([]byte(evt.Lease.Data), &attrs); err != nil {
					log.Error("Error decoding subnet lease JSON: ", err)
					continue
				}
				rtr.SetRoute(evt.Lease.Network, attrs.PublicIP)

			} else if evt.Type == subnet.SubnetRemoved {
				log.Info("Subnet removed: %v", evt.Lease.Network)
				rtr.DelRoute(evt.Lease.Network)

			} else {
				log.Errorf("Internal error: unknown event type: %d", int(evt.Type))
			}
		}
	}
}

func Run(sm *subnet.SubnetManager, iface *net.Interface, ip net.IP, port int, ready backend.ReadyFunc) {
	sn, err := acquireLease(sm, ip)
	if err != nil {
		log.Error("Failed to acquire lease: ", err)
		return
	}

	tun, tunName, err := pkg.OpenTun("kolach%d")
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
		mtu = defaultMTU
	}

	err = configureIface(tunName, ipn, mtu)
	if err != nil {
		return
	}

	rtr := NewRouter(port)

	// all initialized and ready for business
	log.Info("UDP encapsulation initialized")
	ready(sn, mtu)

	log.Info("Dispatching to run the proxy loop")
	go proxyTunToUdp(rtr, tun, conn)
	go proxyUdpToTun(conn, tun)

	log.Info("Watching for new subnet leases")
	monitorEvents(sm, rtr)
}
