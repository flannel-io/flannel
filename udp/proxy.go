package udp

import (
	"encoding/json"
	"net"
	"os"
	"sync"

	log "github.com/coreos/rudder/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/rudder/pkg/ip"
	"github.com/coreos/rudder/subnet"
)

const (
	minIP4HdrSize = 20
)

type routeEntry struct {
	sn   ip.IP4Net
	addr *net.UDPAddr
}

type Router struct {
	mux    sync.Mutex
	port   int
	routes []routeEntry
}

func NewRouter(port int) *Router {
	return &Router{
		port: port,
	}
}

func (r *Router) SetRoute(sn ip.IP4Net, dst ip.IP4) {
	r.mux.Lock()
	defer r.mux.Unlock()

	for _, re := range r.routes {
		if re.sn.Equal(sn) {
			re.addr = &net.UDPAddr{
				IP:   dst.ToIP(),
				Port: r.port,
			}
			return
		}
	}

	re := routeEntry{
		sn: sn,
		addr: &net.UDPAddr{
			IP:   dst.ToIP(),
			Port: r.port,
		},
	}

	r.routes = append(r.routes, re)
}

func (r *Router) DelRoute(sn ip.IP4Net) {
	r.mux.Lock()
	defer r.mux.Unlock()

	for i, re := range r.routes {
		if re.sn.Equal(sn) {
			r.routes[i] = r.routes[len(r.routes)-1]
			r.routes = r.routes[:len(r.routes)-1]
			return
		}
	}
}

func (r *Router) routePacket(pkt []byte, conn *net.UDPConn) {
	if len(pkt) < minIP4HdrSize {
		log.V(1).Infof("Packet too small (%d bytes), unable to route", len(pkt))
		return
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	dstIP := ip.FromBytes(pkt[16:20])

	for i, re := range r.routes {
		if re.sn.Contains(dstIP) {
			nbytes, err := conn.WriteToUDP(pkt, re.addr)
			switch {
			case err != nil:
				log.V(1).Info("UDP send failed with: ", err)
			case nbytes != len(pkt):
				log.V(1).Infof("Was only able to UDP send %d out of %d bytes to %s: ", nbytes, len(pkt), re.addr.IP)
			}

			// packets for same dest tend to come in burst. swap to front make it faster for subsequent ones
			if i != 0 {
				r.routes[0], r.routes[i] = r.routes[i], r.routes[0]
			}
			return
		}
	}

	log.V(1).Info("No route found for ", dstIP)
}

func proxy(sm *subnet.SubnetManager, tun *os.File, conn *net.UDPConn, tunMTU uint, port int) {
	log.Info("Running slow proxy loop")

	rtr := NewRouter(port)

	go proxyTunToUdp(rtr, tun, conn, tunMTU)
	go proxyUdpToTun(conn, tun, tunMTU)

	log.Info("Watching for new subnet leases")
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
				log.Info("Subnet removed: ", evt.Lease.Network)
				rtr.DelRoute(evt.Lease.Network)

			} else {
				log.Error("Internal error: unknown event type: ", int(evt.Type))
			}
		}
	}
}

func proxyTunToUdp(r *Router, tun *os.File, conn *net.UDPConn, tunMTU uint) {
	pkt := make([]byte, tunMTU)
	for {
		nbytes, err := tun.Read(pkt)
		if err != nil {
			log.V(1).Info("Error reading from TUN device: ", err)
		} else {
			r.routePacket(pkt[:nbytes], conn)
		}
	}
}

func proxyUdpToTun(conn *net.UDPConn, tun *os.File, tunMTU uint) {
	pkt := make([]byte, tunMTU)
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
