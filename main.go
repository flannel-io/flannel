package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/coreos/go-systemd/daemon"
	log "github.com/coreos/rudder/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/rudder/backend"
	"github.com/coreos/rudder/pkg/ip"
	"github.com/coreos/rudder/subnet"
	"github.com/coreos/rudder/backend/udp"
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

func writeSubnetFile(sn ip.IP4Net, mtu int) error {
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

func lookupIface() (*net.Interface, net.IP, error) {
	var iface *net.Interface
	var ipaddr net.IP
	var err error

	if len(opts.iface) > 0 {
		if ipaddr = net.ParseIP(opts.iface); ipaddr != nil {
			iface, err = ip.GetInterfaceByIP(ipaddr)
			if err != nil {
				return nil, nil, fmt.Errorf("Error looking up interface %s: %s", opts.iface, err)
			}
		} else {
			iface, err = net.InterfaceByName(opts.iface)
			if err != nil {
				return nil, nil, fmt.Errorf("Error looking up interface %s: %s", opts.iface, err)
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

	if ipaddr == nil {
		ipaddr, err = ip.GetIfaceIP4Addr(iface)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to find IPv4 address for interface %s", iface.Name)
		}
	}

	return iface, ipaddr, nil
}

func makeSubnetManager() *subnet.SubnetManager {
	for {
		sm, err := subnet.NewSubnetManager(opts.etcdEndpoint, opts.etcdPrefix)
		if err == nil {
			return sm
		}

		log.Error("Failed to create SubnetManager: ", err)
		time.Sleep(time.Second)
	}
}

func newBackend() backend.Backend {
	sm := makeSubnetManager()
	return udp.New(sm, opts.port)
}

func run(be backend.Backend, quit chan bool) {
	defer close(quit)

	iface, ipaddr, err := lookupIface()
	if err != nil {
		log.Error(err)
		return
	}

	if iface.MTU == 0 {
		log.Errorf("Failed to determine MTU for %s interface", ipaddr)
		return
	}

	log.Infof("Using %s as external interface", ipaddr)

	sn, mtu, err := be.Init(iface, ipaddr, opts.ipMasq)
	if err != nil {
		log.Error(err)
		return
	}

	writeSubnetFile(sn, mtu)
	daemon.SdNotify("READY=1")

	log.Infof("%s mode initialized", be.Name())
	be.Run()
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

	be := newBackend()

	// Register for SIGINT and SIGTERM and wait for one of them to arrive
	log.Info("Installing signal handlers")
	sigs := make(chan os.Signal, 5)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	quit := make(chan bool)
	go run(be, quit)

	for {
		select {
		case <-sigs:
			// unregister to get default OS nuke behaviour in case we don't exit cleanly
			signal.Stop(sigs)

			log.Info("Exiting...")
			be.Stop()

		case <-quit:
			log.Infof("%s mode exited", be.Name())
			os.Exit(0)
		}
	}
}
