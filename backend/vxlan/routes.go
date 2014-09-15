package vxlan

import (
	"bytes"
	"net"

	"github.com/coreos/flannel/pkg/ip"
)

type route struct {
	network ip.IP4Net
	vtepIP  net.IP
	vtepMAC net.HardwareAddr
}

type routes []route

func (rts *routes) set(nw ip.IP4Net, vtepIP net.IP, vtepMAC net.HardwareAddr) {
	for i, rt := range *rts {
		if rt.network.Equal(nw) {
			(*rts)[i].vtepIP = vtepIP
			(*rts)[i].vtepMAC = vtepMAC
			return
		}
	}
	*rts = append(*rts, route{nw, vtepIP, vtepMAC})
}

func (rts *routes) remove(nw ip.IP4Net) {
	for i, rt := range *rts {
		if rt.network.Equal(nw) {
			(*rts)[i] = (*rts)[len(*rts)-1]
			(*rts) = (*rts)[0 : len(*rts)-1]
			return
		}
	}
}

func (rts routes) findByNetwork(ipAddr ip.IP4) *route {
	for i, rt := range rts {
		if rt.network.Contains(ipAddr) {
			return &rts[i]
		}
	}
	return nil
}

func (rts routes) findByVtepMAC(mac net.HardwareAddr) *route {
	for i, rt := range rts {
		if bytes.Equal(rt.vtepMAC, mac) {
			return &rts[i]
		}
	}
	return nil
}
