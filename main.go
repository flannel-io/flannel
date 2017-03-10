// Copyright 2015 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/coreos/pkg/flagutil"
	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/network"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
	"github.com/coreos/flannel/subnet/etcdv2"
	"github.com/coreos/flannel/subnet/kube"
	"github.com/coreos/flannel/version"

	"time"

	// Backends need to be imported for their init() to get executed and them to register
	"github.com/coreos/flannel/backend"
	_ "github.com/coreos/flannel/backend/alivpc"
	_ "github.com/coreos/flannel/backend/alloc"
	_ "github.com/coreos/flannel/backend/awsvpc"
	_ "github.com/coreos/flannel/backend/gce"
	_ "github.com/coreos/flannel/backend/hostgw"
	_ "github.com/coreos/flannel/backend/udp"
	_ "github.com/coreos/flannel/backend/vxlan"
	"github.com/coreos/go-systemd/daemon"
)

type CmdLineOpts struct {
	etcdEndpoints          string
	etcdPrefix             string
	etcdKeyfile            string
	etcdCertfile           string
	etcdCAFile             string
	etcdUsername           string
	etcdPassword           string
	help                   bool
	version                bool
	kubeSubnetMgr          bool
	iface                  string
	ipMasq                 bool
	subnetFile             string
	subnetDir              string
	publicIP               string
	subnetLeaseRenewMargin int
}

var (
	opts           CmdLineOpts
	errInterrupted = errors.New("interrupted")
	errCanceled    = errors.New("canceled")
)

func init() {
	flag.StringVar(&opts.etcdEndpoints, "etcd-endpoints", "http://127.0.0.1:4001,http://127.0.0.1:2379", "a comma-delimited list of etcd endpoints")
	flag.StringVar(&opts.etcdPrefix, "etcd-prefix", "/coreos.com/network", "etcd prefix")
	flag.StringVar(&opts.etcdKeyfile, "etcd-keyfile", "", "SSL key file used to secure etcd communication")
	flag.StringVar(&opts.etcdCertfile, "etcd-certfile", "", "SSL certification file used to secure etcd communication")
	flag.StringVar(&opts.etcdCAFile, "etcd-cafile", "", "SSL Certificate Authority file used to secure etcd communication")
	flag.StringVar(&opts.etcdUsername, "etcd-username", "", "Username for BasicAuth to etcd")
	flag.StringVar(&opts.etcdPassword, "etcd-password", "", "Password for BasicAuth to etcd")
	flag.StringVar(&opts.iface, "iface", "", "interface to use (IP or name) for inter-host communication")
	flag.StringVar(&opts.subnetFile, "subnet-file", "/run/flannel/subnet.env", "filename where env variables (subnet, MTU, ... ) will be written to")
	flag.StringVar(&opts.publicIP, "public-ip", "", "IP accessible by other nodes for inter-host communication")
	flag.IntVar(&opts.subnetLeaseRenewMargin, "subnet-lease-renew-margin", 60, "Subnet lease renewal margin, in minutes.")
	flag.BoolVar(&opts.ipMasq, "ip-masq", false, "setup IP masquerade rule for traffic destined outside of overlay network")
	flag.BoolVar(&opts.kubeSubnetMgr, "kube-subnet-mgr", false, "Contact the Kubernetes API for subnet assignement instead of etcd or flannel-server.")
	flag.BoolVar(&opts.help, "help", false, "print this message")
	flag.BoolVar(&opts.version, "version", false, "print version and exit")
}

func newSubnetManager() (subnet.Manager, error) {
	if opts.kubeSubnetMgr {
		return kube.NewSubnetManager()
	}

	cfg := &etcdv2.EtcdConfig{
		Endpoints: strings.Split(opts.etcdEndpoints, ","),
		Keyfile:   opts.etcdKeyfile,
		Certfile:  opts.etcdCertfile,
		CAFile:    opts.etcdCAFile,
		Prefix:    opts.etcdPrefix,
		Username:  opts.etcdUsername,
		Password:  opts.etcdPassword,
	}

	return etcdv2.NewLocalManager(cfg)
}

func main() {
	// glog will log to tmp files by default. override so all entries
	// can flow into journald (if running under systemd)
	flag.Set("logtostderr", "true")

	// now parse command line args
	flag.Parse()

	if flag.NArg() > 0 || opts.help {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	if opts.version {
		fmt.Fprintln(os.Stderr, version.Version)
		os.Exit(0)
	}

	flagutil.SetFlagsFromEnv(flag.CommandLine, "FLANNELD")

	// Work out which interface to use
	extIface, err := LookupExtIface(opts.iface)
	if err != nil {
		log.Error("Failed to find interface to use: ", err)
		os.Exit(1)
	}

	sm, err := newSubnetManager()
	if err != nil {
		log.Error("Failed to create SubnetManager: ", err)
		os.Exit(1)
	}

	// Register for SIGINT and SIGTERM
	log.Info("Installing signal handlers")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go shutdown(sigs, cancel)

	// Fetch the network config (i.e. what backend to use etc..).
	config, err := getConfig(ctx, sm)
	if err == errCanceled {
		exit()
	}

	// Create a backend manager then use it to create the backend and register the network with it.
	bm := backend.NewManager(ctx, sm, extIface)
	be, err := bm.GetBackend(config.BackendType)
	if err != nil {
		log.Errorf("Error fetching backend: %s", err)
		exit()
	}

	bn, err := be.RegisterNetwork(ctx, config)
	if err != nil {
		log.Errorf("Error registering network: %s", err)
		exit()
	}

	// Set up ipMasq if needed
	if opts.ipMasq {
		err = network.SetupIPMasq(config.Network)
		if err != nil {
			// Continue, even though it failed.
			log.Errorf("Failed to set up IP Masquerade: %v", err)
		}

		defer func() {
			if err := network.TeardownIPMasq(config.Network); err != nil {
				log.Errorf("Failed to tear down IP Masquerade: %v", err)
			}
		}()
	}

	if err := WriteSubnetFile(opts.subnetFile, config.Network, opts.ipMasq, bn); err != nil {
		// Continue, even though it failed.
		log.Warningf("Failed to write subnet file: %s", err)
	} else {
		log.Infof("Wrote subnet file to %s", opts.subnetFile)
	}

	// Start "Running" the backend network. This will block until the context is done so run in another goroutine.
	go bn.Run(ctx)
	log.Infof("Finished starting backend.")

	daemon.SdNotify(false, "READY=1")

	// Block waiting to renew the lease
	_ = MonitorLease(ctx, sm, bn)

	// To get to here, the Cancel signal must have been received or the lease has been revoked.
	exit()
}

func exit() {
	// Wait just a second for the cancel signal to propagate everywhere, then just exit cleanly.
	log.Info("Waiting for cancel to propagate...")
	time.Sleep(time.Second)
	log.Info("Exiting...")
	os.Exit(0)
}

func shutdown(sigs chan os.Signal, cancel context.CancelFunc) {
	// Wait for the shutdown signal.
	<-sigs
	// Unregister to get default OS nuke behaviour in case we don't exit cleanly
	signal.Stop(sigs)
	log.Info("Starting shutdown...")

	// Call cancel on the context to close everything down.
	cancel()
	log.Info("Sent cancel signal...")
}

func getConfig(ctx context.Context, sm subnet.Manager) (*subnet.Config, error) {
	// Retry every second until it succeeds
	for {
		config, err := sm.GetNetworkConfig(ctx)
		if err != nil {
			log.Errorf("Couldn't fetch network config: %s", err)
		} else if config == nil {
			log.Warningf("Couldn't find network config: %s", err)
		} else {
			log.Infof("Found network config - Backend type: %s", config.BackendType)
			return config, nil
		}
		select {
		case <-ctx.Done():
			return nil, errCanceled
		case <-time.After(1 * time.Second):
			fmt.Println("timed out")
		}
	}
}

func MonitorLease(ctx context.Context, sm subnet.Manager, bn backend.Network) error {
	// Use the subnet manager to start watching leases.
	evts := make(chan subnet.Event)
	go subnet.WatchLease(ctx, sm, bn.Lease().Subnet, evts)
	renewMargin := time.Duration(opts.subnetLeaseRenewMargin) * time.Minute
	dur := bn.Lease().Expiration.Sub(time.Now()) - renewMargin

	for {
		select {
		case <-time.After(dur):
			err := sm.RenewLease(ctx, bn.Lease())
			if err != nil {
				log.Error("Error renewing lease (trying again in 1 min): ", err)
				dur = time.Minute
				continue
			}

			log.Info("Lease renewed, new expiration: ", bn.Lease().Expiration)
			dur = bn.Lease().Expiration.Sub(time.Now()) - renewMargin

		case e := <-evts:
			switch e.Type {
			case subnet.EventAdded:
				bn.Lease().Expiration = e.Lease.Expiration
				dur = bn.Lease().Expiration.Sub(time.Now()) - renewMargin
				log.Infof("Waiting for %s to renew lease", dur)

			case subnet.EventRemoved:
				log.Error("Lease has been revoked. Shutting down daemon.")
				return errInterrupted
			}

		case <-ctx.Done():
			log.Infof("Stopped monitoring lease")
			return errCanceled
		}
	}
}

func LookupExtIface(ifname string) (*backend.ExternalInterface, error) {
	var iface *net.Interface
	var ifaceAddr net.IP
	var err error

	if len(ifname) > 0 {
		if ifaceAddr = net.ParseIP(ifname); ifaceAddr != nil {
			log.Infof("Searching for interface using %s", ifaceAddr)
			iface, err = ip.GetInterfaceByIP(ifaceAddr)
			if err != nil {
				return nil, fmt.Errorf("error looking up interface %s: %s", ifname, err)
			}
		} else {
			iface, err = net.InterfaceByName(ifname)
			if err != nil {
				return nil, fmt.Errorf("error looking up interface %s: %s", ifname, err)
			}
		}
	} else {
		log.Info("Determining IP address of default interface")
		if iface, err = ip.GetDefaultGatewayIface(); err != nil {
			return nil, fmt.Errorf("failed to get default interface: %s", err)
		}
	}

	if ifaceAddr == nil {
		ifaceAddr, err = ip.GetIfaceIP4Addr(iface)
		if err != nil {
			return nil, fmt.Errorf("failed to find IPv4 address for interface %s", iface.Name)
		}
	}

	log.Infof("Using interface with name %s and address %s", iface.Name, ifaceAddr)

	if iface.MTU == 0 {
		return nil, fmt.Errorf("failed to determine MTU for %s interface", ifaceAddr)
	}

	var extAddr net.IP

	if len(opts.publicIP) > 0 {
		extAddr = net.ParseIP(opts.publicIP)
		if extAddr == nil {
			return nil, fmt.Errorf("invalid public IP address: %s", opts.publicIP)
		}
		log.Infof("Using %s as external address", extAddr)
	}

	if extAddr == nil {
		log.Infof("Defaulting external address to interface address (%s)", ifaceAddr)
		extAddr = ifaceAddr
	}

	return &backend.ExternalInterface{
		Iface:     iface,
		IfaceAddr: ifaceAddr,
		ExtAddr:   extAddr,
	}, nil
}

func WriteSubnetFile(path string, nw ip.IP4Net, ipMasq bool, bn backend.Network) error {
	dir, name := filepath.Split(path)
	os.MkdirAll(dir, 0755)

	tempFile := filepath.Join(dir, "."+name)
	f, err := os.Create(tempFile)
	if err != nil {
		return err
	}

	// Write out the first usable IP by incrementing
	// sn.IP by one
	sn := bn.Lease().Subnet
	sn.IP += 1

	fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", nw)
	fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn)
	fmt.Fprintf(f, "FLANNEL_MTU=%d\n", bn.MTU())
	_, err = fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq)
	f.Close()
	if err != nil {
		return err
	}

	// rename(2) the temporary file to the desired location so that it becomes
	// atomically visible with the contents
	return os.Rename(tempFile, path)
	//TODO - is this safe? What if it's not on the same FS?
}
