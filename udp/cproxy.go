package udp

//#include "proxy.h"
import "C"

import (
	"encoding/json"
	"net"
	"os"
	"reflect"
	"syscall"
	"unsafe"

	log "github.com/coreos/rudder/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/rudder/pkg/ip"
	"github.com/coreos/rudder/subnet"
)

func runCProxy(tun *os.File, conn *os.File, ctl *os.File, tunIP ip.IP4, tunMTU uint) {
	var log_errors int
	if log.V(1) {
		log_errors = 1
	}

	C.run_proxy(
		C.int(tun.Fd()),
		C.int(conn.Fd()),
		C.int(ctl.Fd()),
		C.in_addr_t(tunIP.NetworkOrder()),
		C.size_t(tunMTU),
		C.int(log_errors),
	)
}

func writeCommand(f *os.File, cmd *C.command) {
	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(cmd)),
		Len:  int(unsafe.Sizeof(*cmd)),
		Cap:  int(unsafe.Sizeof(*cmd)),
	}
	buf := *(*[]byte)(unsafe.Pointer(&hdr))

	f.Write(buf)
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

func fastProxy(sm *subnet.SubnetManager, tun *os.File, conn *net.UDPConn, tunIP ip.IP4, tunMTU uint, port int) {
	log.Info("Running fast proxy loop")

	c, err := conn.File()
	if err != nil {
		log.Error("Converting UDPConn to File failed: ", err)
		return
	}
	defer c.Close()

	ctl, peerCtl, err := newCtlSockets()
	if err != nil {
		log.Error("Failed to create control socket: ", err)
		return
	}
	defer ctl.Close()
	defer peerCtl.Close()

	go runCProxy(tun, c, peerCtl, tunIP, tunMTU)

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

				cmd := C.command{
					cmd:           C.CMD_SET_ROUTE,
					dest_net:      C.in_addr_t(evt.Lease.Network.IP.NetworkOrder()),
					dest_net_len:  C.int(evt.Lease.Network.PrefixLen),
					next_hop_ip:   C.in_addr_t(attrs.PublicIP.NetworkOrder()),
					next_hop_port: C.short(port),
				}

				writeCommand(ctl, &cmd)

			} else if evt.Type == subnet.SubnetRemoved {
				log.Info("Subnet removed: ", evt.Lease.Network)

				cmd := C.command{
					cmd:          C.CMD_DEL_ROUTE,
					dest_net:     C.in_addr_t(evt.Lease.Network.IP.NetworkOrder()),
					dest_net_len: C.int(evt.Lease.Network.PrefixLen),
				}

				writeCommand(ctl, &cmd)

			} else {
				log.Error("Internal error: unknown event type: ", int(evt.Type))
			}
		}
	}
}
