package ipsec

import (
	"fmt"
	"io/ioutil"

	"github.com/vishvananda/netlink"

	"github.com/coreos/flannel/subnet"
	log "github.com/golang/glog"
	"golang.org/x/net/context"
)

const k8sIPVSIfname = "kube-ipvs0"

/*
IPVS proxy help:
When connecting to a virtual server from the host, the packet will have src=ClusterIP dst=ClusterIP
This will be translated to src=ClusterIP dst=PodIP by the IPVS module.
This packet will be masqueraded by flannel, since it can't decide which IP to use, it uses the
**public** IP address, since the route for the remote pod network points into the public network interface.

Now there are several options:
- Add the PublicIP <-> Remote Pod Subnet to the IPSec policies which will lead to NAT/Masquerade bugs within IPTables
- Change the source address for virtual services.

I chose the second approach, since it can be automated and is trivial to understand.
With this hack established, the protocol above looks like this:
src=CNI-IP dst=ClusterIP
which will be translated by IPVS to src=CNI-IP dst=PodIP and forwarded correctly.
The other side will send back the packets via the IPSec tunnel and we're good to route them back to IPVS/our CNI/lo interface.

Additionally, we need to disable IPSec policies on the CNI interface to prevent the kernel from re-framing the
arrived packets.
This is done in the functions below.
*/

func MonitorIPVSInterface(ctx context.Context, lease *subnet.Lease) {
	log.Infof("Starting route manager for ipvs interface")
	updateChan := make(chan netlink.AddrUpdate)
	if err := netlink.AddrSubscribeWithOptions(updateChan, ctx.Done(), netlink.AddrSubscribeOptions{
		ListExisting: true,
	}); err != nil {
		log.Errorf("Failed to subscribe to address changes: %v", err)
	}
	src := lease.Subnet.IP.ToIP()
	src[len(src)-1]++ // Pick the first valid IP address from the assigned prefix, which is the address of the CNI interface

	for evt := range updateChan {
		if !evt.NewAddr {
			// Address was deleted from the interface, so skip all processing and just wait for the next event.
			// When an address is deleted, the routes are cleaned up from the local routing table automatically
			// by the kernel, so we don't have anything to do here.
			continue
		}

		if link, err := netlink.LinkByIndex(evt.LinkIndex); err != nil || link.Attrs().Name != k8sIPVSIfname {
			// A new address was added to a link, but either the link couldn't be found or
			// it wasn't the IPVS link, so skip it.
			continue
		} else {
			log.Infof("Received update for IPVS link, changing route")
			if err := netlink.RouteReplace(&netlink.Route{
				LinkIndex: evt.LinkIndex,
				Table:     255, // Local Routing Table
				Dst:       &evt.LinkAddress,
				Type:      2, // local route.
				Src:       src,
				Scope:     netlink.SCOPE_HOST,
				Protocol:  2, // Proto Kernel
			}); err != nil {
				log.Warningf("Failed to replace route: %v", err)
			}
		}

	}
	log.Infof("Stopped route manager")
}

func MonitorCNIInterface(ctx context.Context, ifname string) {
	// This function takes care of disabling the IPSec policy for the CNI0 interface.
	log.Infof("Starting CNI Monitor for interface '%s'", ifname)
	updateChan := make(chan netlink.LinkUpdate)
	if err := netlink.LinkSubscribeWithOptions(updateChan, ctx.Done(), netlink.LinkSubscribeOptions{
		ListExisting: true,
	}); err != nil {
		log.Errorf("Failed to subscribe to link ")
	}
	disablePolicyPath := fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/disable_policy", ifname)
	for evt := range updateChan {
		if evt.Attrs().Name != ifname {
			continue
		}

		log.Infof("CNI link event detected, disabling IPSec policy")
		if err := ioutil.WriteFile(disablePolicyPath, []byte("1"), 644); err != nil {
			log.Warningf("Failed to disable policy for CNI link %s: %v", ifname, err)
		}

	}
	log.Infof("CNI Monitor stopped for interface '%s'", ifname)
}
