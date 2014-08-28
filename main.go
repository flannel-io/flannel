package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/coreos/go-systemd/daemon"
	log "github.com/coreos/rudder/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/rudder/pkg/ip"
	"github.com/coreos/rudder/subnet"
	"github.com/coreos/rudder/udp"
)

const (
	defaultPort = 8285
)

type CmdLineOpts struct {
	etcdEndpoint string
	etcdPrefix   string
	help         bool
	version      bool
	ipMasq       bool
	port         int
	subnetFile   string
	iface        string
}

var opts CmdLineOpts

func init() {
	flag.StringVar(&opts.etcdEndpoint, "etcd-endpoint", "http://127.0.0.1:4001", "etcd endpoint")
	flag.StringVar(&opts.etcdPrefix, "etcd-prefix", "/coreos.com/network", "etcd prefix")
	flag.IntVar(&opts.port, "port", defaultPort, "port to use for inter-node communications")
	flag.StringVar(&opts.subnetFile, "subnet-file", "/run/rudder/subnet.env", "filename where env variables (subnet and MTU values) will be written to")
	flag.StringVar(&opts.iface, "iface", "", "interface to use (IP or name) for inter-host communication")
	flag.BoolVar(&opts.ipMasq, "ip-masq", false, "setup IP masquerade rule for traffic destined outside of overlay network")
	flag.BoolVar(&opts.help, "help", false, "print this message")
	flag.BoolVar(&opts.version, "version", false, "print version and exit")
}

func writeSubnet(sn ip.IP4Net, mtu int) error {
	// Write out the first usable IP by incrementing
	// sn.IP by one
	sn.IP += 1

	dir, _ := path.Split(opts.subnetFile)
	os.MkdirAll(dir, 0755)

	f, err := os.Create(opts.subnetFile)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "RUDDER_SUBNET=%s\n", sn)
	fmt.Fprintf(f, "RUDDER_MTU=%d\n", mtu)
	return nil
}

func lookupIface() (*net.Interface, net.IP) {
	var iface *net.Interface
	var tep net.IP
	var err error

	if len(opts.iface) > 0 {
		if tep = net.ParseIP(opts.iface); tep != nil {
			iface, err = ip.GetInterfaceByIP(tep)
			if err != nil {
				log.Errorf("Error looking up interface %s: %s", opts.iface, err)
				return nil, nil
			}
		} else {
			iface, err = net.InterfaceByName(opts.iface)
			if err != nil {
				log.Errorf("Error looking up interface %s: %s", opts.iface, err)
				return nil, nil
			}
		}
	} else {
		log.Info("Determining IP address of default interface")
		for {
			if iface, err = ip.GetDefaultGatewayIface(); err == nil {
				break
			}
			log.Error("Failed to get default interface: ", err)
			time.Sleep(time.Second)
		}
	}

	if tep == nil {
		tep, err = ip.GetIfaceIP4Addr(iface)
		if err != nil {
			log.Error("Failed to find IPv4 address for interface ", iface.Name)
		}
	}

	return iface, tep
}

func makeSubnetManager() *subnet.SubnetManager {
	etcdCli := etcd.NewClient([]string{opts.etcdEndpoint})

	for {
		sm, err := subnet.NewSubnetManager(etcdCli, opts.etcdPrefix)
		if err == nil {
			return sm
		}

		log.Error("Failed to create SubnetManager: ", err)
		time.Sleep(time.Second)
	}
}

func main() {
	// glog will log to tmp files by default. override so all entries
	// can flow into journald (if running under systemd)
	flag.Set("logtostderr", "true")

	// now parse command line args
	flag.Parse()

	if opts.help {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	if opts.version {
		fmt.Fprintln(os.Stderr, Version)
		os.Exit(0)
	}

	iface, tep := lookupIface()
	if iface == nil || tep == nil {
		return
	}

	log.Infof("Using %s to tunnel", tep)

	sm := makeSubnetManager()

	udp.Run(sm, iface, tep, opts.port, opts.ipMasq, func(sn ip.IP4Net, mtu int) {
		writeSubnet(sn, mtu)
		daemon.SdNotify("READY=1")
	})
}
