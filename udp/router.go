package udp

import (
	"net"
	"sync"

	log "github.com/coreos-inc/kolach/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos-inc/kolach/pkg"
)

const (
	minIP4HdrSize = 20
)

type routeEntry struct {
	sn   pkg.IP4Net
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

func (r *Router) SetRoute(sn pkg.IP4Net, dst pkg.IP4) {
	r.mux.Lock()
	defer r.mux.Unlock()

	for _, re := range r.routes {
		if re.sn.Equal(sn) {
			re.addr = &net.UDPAddr{
				IP: dst.ToIP(),
				Port: r.port,
			}
			return
		}
	}

	re := routeEntry{
		sn: sn,
		addr: &net.UDPAddr{
			IP: dst.ToIP(),
			Port: r.port,
		},
	}

	r.routes = append(r.routes, re)
}

func (r *Router) DelRoute(sn pkg.IP4Net) {
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

	dstIP := pkg.FromBytes(pkt[16:20])

	for i, re := range r.routes {
		if re.sn.Contains(dstIP) {
			nbytes, err := conn.WriteToUDP(pkt, re.addr)
			if err != nil || nbytes != len(pkt) {
				if err != nil {
					log.V(1).Info("UDP write failed with: ", err)
				} else {
					log.V(1).Infof("Was only able to send %d out of %d bytes to %s: ", nbytes, len(pkt), re.addr.IP)
				}
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

