package main

import (
	"fmt"
	"net"
	"os"
	"time"
	"flag"
	"path"

	"github.com/coreos-inc/kolach/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos-inc/kolach/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos-inc/kolach/Godeps/_workspace/src/github.com/coreos/go-systemd/daemon"

	"github.com/coreos-inc/kolach/pkg"
	"github.com/coreos-inc/kolach/subnet"
	"github.com/coreos-inc/kolach/udp"
)

const (
	defaultPort = 4242
)

type CmdLineOpts struct {
	etcdEndpoint string
	etcdPrefix   string
	help         bool
	version      bool
	port         int
	subnetFile   string
	iface        string
}

var opts CmdLineOpts

func init() {
	flag.StringVar(&opts.etcdEndpoint, "etcd-endpoint", "http://127.0.0.1:4001", "etcd endpoint")
	flag.StringVar(&opts.etcdPrefix, "etcd-prefix", "/coreos.com/network", "etcd prefix")
	flag.IntVar(&opts.port, "port", defaultPort, "port to use for inter-node communications")
	flag.StringVar(&opts.subnetFile, "subnet-file", "/run/kolach/subnet.env", "filename where env variables (subnet and MTU values) will be written to")
	flag.StringVar(&opts.iface, "iface", "", "interface to use (IP or name) for inter-host communication")
	flag.BoolVar(&opts.help, "help", false, "print this message")
	flag.BoolVar(&opts.version, "version", false, "print version and exit")
}

func writeSubnet(sn pkg.IP4Net, mtu int) error {
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

	fmt.Fprintf(f, "KOLACH_SUBNET=%s\n", sn)
	fmt.Fprintf(f, "KOLACH_MTU=%d\n", mtu)
	return nil
}

func lookupIface() (*net.Interface, net.IP) {
	var iface *net.Interface
	var ip net.IP
	var err error

	if len(opts.iface) > 0 {
		if ip = net.ParseIP(opts.iface); ip != nil {
			iface, err = pkg.GetInterfaceByIP(ip)
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
			if iface, err = pkg.GetDefaultGatewayIface(); err == nil {
				break
			}
			log.Error("Failed to get default interface: ", err)
			time.Sleep(time.Second)
		}
	}

	if ip == nil {
		ip, err = pkg.GetIfaceIP4Addr(iface)
		if err != nil {
			log.Error("Failed to find IPv4 address for interface ", iface.Name)
		}
	}

	return iface, ip
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

	iface, ip := lookupIface()
	if iface == nil || ip == nil {
		return
	}

	log.Infof("Using %s to tunnel", ip)

	sm := makeSubnetManager()

	udp.Run(sm, iface, ip, opts.port, func(sn pkg.IP4Net, mtu int) {
		writeSubnet(sn, mtu)
		daemon.SdNotify("READY=1")
	})
}
