package udp

import (
	"fmt"
	"encoding/json"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/docker/libcontainer/netlink"
	log "github.com/coreos/rudder/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/rudder/backend"
	"github.com/coreos/rudder/pkg/ip"
	"github.com/coreos/rudder/pkg/task"
	"github.com/coreos/rudder/subnet"
)

const (
	encapOverhead = 28 // 20 bytes IP hdr + 8 bytes UDP hdr
	defaultPort   = 8285
)

type UdpBackend struct {
	sm     *subnet.SubnetManager
	rawCfg json.RawMessage
	cfg    struct {
		Port int
	}
	ctl    *os.File
	ctl2   *os.File
	tun    *os.File
	conn   *net.UDPConn
	mtu    int
	tunNet ip.IP4Net
	stop   chan bool
	wg     sync.WaitGroup
}

func New(sm *subnet.SubnetManager, config json.RawMessage) backend.Backend {
	be := UdpBackend{
		sm:     sm,
		rawCfg: config,
		stop:   make(chan bool),
	}
	be.cfg.Port = defaultPort
	return &be
}

func (m *UdpBackend) Init(extIface *net.Interface, extIP net.IP, ipMasq bool) (ip.IP4Net, int, error) {
	// Parse our configuration
	if len(m.rawCfg) > 0 {
		if err := json.Unmarshal(m.rawCfg, &m.cfg); err != nil {
			return ip.IP4Net{}, 0, fmt.Errorf("Error decoding UDP backend config: %v", err)
		}
	}

	// Acquire the lease form subnet manager
	attrs := subnet.BaseAttrs{
		PublicIP: ip.FromIP(extIP),
	}

	sn, err := m.sm.AcquireLease(attrs.PublicIP, &attrs, m.stop)
	if err != nil {
		if err == task.ErrCanceled {
			return ip.IP4Net{}, 0, err
		} else {
			return ip.IP4Net{}, 0, fmt.Errorf("Failed to acquire lease: %s", err)
		}
	}

	// Tunnel's subnet is that of the whole overlay network (e.g. /16)
	// and not that of the individual host (e.g. /24)
	m.tunNet = ip.IP4Net{
		IP:        sn.IP,
		PrefixLen: m.sm.GetConfig().Network.PrefixLen,
	}

	// TUN MTU will be smaller b/c of encap (IP+UDP hdrs)
	m.mtu = extIface.MTU - encapOverhead

	if err = m.initTun(ipMasq); err != nil {
		return ip.IP4Net{}, 0, err
	}

	m.conn, err = net.ListenUDP("udp4", &net.UDPAddr{Port: m.cfg.Port})
	if err != nil {
		return ip.IP4Net{}, 0, fmt.Errorf("Failed to start listening on UDP socket: %s", err)
	}


	m.ctl, m.ctl2, err = newCtlSockets()
	if err != nil {
		return ip.IP4Net{}, 0, fmt.Errorf("Failed to create control socket: %s", err)
	}

	return sn, m.mtu, nil
}

func (m *UdpBackend) Run() {
	// one for each goroutine below
	m.wg.Add(2)

	go func() {
		runCProxy(m.tun, m.conn, m.ctl2, m.tunNet.IP, m.mtu)
		m.wg.Done()
	}()

	go func() {
		m.sm.LeaseRenewer(m.stop)
		m.wg.Done()
	}()

	m.monitorEvents()

	m.wg.Wait()
}

func (m *UdpBackend) Stop() {
	if m.ctl != nil {
		stopProxy(m.ctl)
	}

	close(m.stop)
}

func (m *UdpBackend) Name() string {
	return "UDP"
}

func newCtlSockets() (*os.File, *os.File, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET, 0)
	if err != nil {
		return nil, nil, err
	}

	f1 := os.NewFile(uintptr(fds[0]), "ctl")
	f2 := os.NewFile(uintptr(fds[1]), "ctl")
	return f1, f2, nil
}

func (m *UdpBackend) initTun(ipMasq bool) error {
	var tunName string
	var err error

	m.tun, tunName, err = ip.OpenTun("rudder%d")
	if err != nil {
		log.Error("Failed to open TUN device: ", err)
		return err
	}

	err = configureIface(tunName, m.tunNet, m.mtu)
	if err != nil {
		return err
	}

	if ipMasq {
		err = setupIpMasq(m.tunNet.Network(), tunName)
		if err != nil {
			return err
		}
	}

	return nil
}

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
		log.Errorf("Failed to set MTU for %s: ", ifname, err)
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
		log.Errorf("Failed to add route (%s -> %s): ", ipn.Network().String(), ifname, err)
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
		[]string{ "RUDDER", "-d", ipn.String(), "-j", "ACCEPT" },
		// This rule makes sure we don't NAT multicast traffic within overlay network
		[]string{ "RUDDER", "-d", "224.0.0.0/4", "-j", "ACCEPT" },
		// This rule will NAT everything originating from our overlay network and 
		[]string{ "RUDDER", "!", "-o", iface, "-j", "MASQUERADE" },
		// This rule will take everything coming from overlay and sent it to RUDDER chain
		[]string{ "POSTROUTING", "-s", ipn.String(), "-j", "RUDDER" },
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
func (m *UdpBackend) monitorEvents() {
	log.Info("Watching for new subnet leases")

	evts := make(chan subnet.EventBatch)

	m.wg.Add(1)
	go func() {
		m.sm.WatchLeases(evts, m.stop)
		m.wg.Done()
	}()

	for {
		select {
		case evtBatch := <-evts:
			for _, evt := range evtBatch {
				switch evt.Type {
				case subnet.SubnetAdded:
					log.Info("Subnet added: ", evt.Lease.Network)

					var attrs subnet.BaseAttrs
					if err := json.Unmarshal([]byte(evt.Lease.Data), &attrs); err != nil {
						log.Error("Error decoding subnet lease JSON: ", err)
						continue
					}

					setRoute(m.ctl, evt.Lease.Network, attrs.PublicIP, m.cfg.Port)

				case subnet.SubnetRemoved:
					log.Info("Subnet removed: ", evt.Lease.Network)

					removeRoute(m.ctl, evt.Lease.Network)

				default:
					log.Error("Internal error: unknown event type: ", int(evt.Type))
				}
			}

		case <-m.stop:
			return
		}
	}
}
