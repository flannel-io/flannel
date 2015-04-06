// Copyright 2015 CoreOS, Inc.
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
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/go-systemd/daemon"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/backend/alloc"
	"github.com/coreos/flannel/backend/hostgw"
	"github.com/coreos/flannel/backend/udp"
	"github.com/coreos/flannel/backend/vxlan"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/pkg/task"
	"github.com/coreos/flannel/subnet"
)

type CmdLineOpts struct {
	etcdEndpoints string
	etcdPrefix    string
	etcdKeyfile   string
	etcdCertfile  string
	etcdCAFile    string
	help          bool
	version       bool
	ipMasq        bool
	subnetFile    string
	iface         string
}

var opts CmdLineOpts

func init() {
	flag.StringVar(&opts.etcdEndpoints, "etcd-endpoints", "http://127.0.0.1:4001,http://localhost:2379", "a comma-delimited list of etcd endpoints")
	flag.StringVar(&opts.etcdPrefix, "etcd-prefix", "/coreos.com/network", "etcd prefix")
	flag.StringVar(&opts.etcdKeyfile, "etcd-keyfile", "", "SSL key file used to secure etcd communication")
	flag.StringVar(&opts.etcdCertfile, "etcd-certfile", "", "SSL certification file used to secure etcd communication")
	flag.StringVar(&opts.etcdCAFile, "etcd-cafile", "", "SSL Certificate Authority file used to secure etcd communication")
	flag.StringVar(&opts.subnetFile, "subnet-file", "/run/flannel/subnet.env", "filename where env variables (subnet and MTU values) will be written to")
	flag.StringVar(&opts.iface, "iface", "", "interface to use (IP or name) for inter-host communication")
	flag.BoolVar(&opts.ipMasq, "ip-masq", false, "setup IP masquerade rule for traffic destined outside of overlay network")
	flag.BoolVar(&opts.help, "help", false, "print this message")
	flag.BoolVar(&opts.version, "version", false, "print version and exit")
}

// TODO: This is yet another copy (others found in etcd, fleet) -- Pull it out!
// flagsFromEnv parses all registered flags in the given flagset,
// and if they are not already set it attempts to set their values from
// environment variables. Environment variables take the name of the flag but
// are UPPERCASE, have the given prefix, and any dashes are replaced by
// underscores - for example: some-flag => PREFIX_SOME_FLAG
func flagsFromEnv(prefix string, fs *flag.FlagSet) {
	alreadySet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		alreadySet[f.Name] = true
	})
	fs.VisitAll(func(f *flag.Flag) {
		if !alreadySet[f.Name] {
			key := strings.ToUpper(prefix + "_" + strings.Replace(f.Name, "-", "_", -1))
			val := os.Getenv(key)
			if val != "" {
				fs.Set(f.Name, val)
			}
		}
	})
}

func writeSubnetFile(sn *backend.SubnetDef) error {
	// Write out the first usable IP by incrementing
	// sn.IP by one
	sn.Net.IP += 1

	dir, name := filepath.Split(opts.subnetFile)
	os.MkdirAll(dir, 0755)

	tempFile := filepath.Join(dir, "."+name)
	f, err := os.Create(tempFile)
	if err != nil {
		return err
	}

	fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn.Net)
	fmt.Fprintf(f, "FLANNEL_MTU=%d\n", sn.MTU)
	_, err = fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", opts.ipMasq)
	f.Close()
	if err != nil {
		return err
	}

	// rename(2) the temporary file to the desired location so that it becomes
	// atomically visible with the contents
	return os.Rename(tempFile, opts.subnetFile)
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
		if iface, err = ip.GetDefaultGatewayIface(); err != nil {
			return nil, nil, fmt.Errorf("Failed to get default interface: %s", err)
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

func newSubnetManager() *subnet.SubnetManager {
	peers := strings.Split(opts.etcdEndpoints, ",")

	cfg := &subnet.EtcdConfig{
		Endpoints: peers,
		Keyfile:   opts.etcdKeyfile,
		Certfile:  opts.etcdCertfile,
		CAFile:    opts.etcdCAFile,
		Prefix:    opts.etcdPrefix,
	}

	for {
		sm, err := subnet.NewSubnetManager(cfg)
		if err == nil {
			return sm
		}

		log.Error("Failed to create SubnetManager: ", err)
		time.Sleep(time.Second)
	}
}

func newBackend(sm *subnet.SubnetManager) (backend.Backend, error) {
	config := sm.GetConfig()

	var bt struct {
		Type string
	}

	if len(config.Backend) == 0 {
		bt.Type = "udp"
	} else {
		if err := json.Unmarshal(config.Backend, &bt); err != nil {
			return nil, fmt.Errorf("Error decoding Backend property of config: %v", err)
		}
	}

	switch strings.ToLower(bt.Type) {
	case "udp":
		return udp.New(sm, config.Backend), nil
	case "alloc":
		return alloc.New(sm), nil
	case "host-gw":
		return hostgw.New(sm), nil
	case "vxlan":
		return vxlan.New(sm, config.Backend), nil
	default:
		return nil, fmt.Errorf("'%v': unknown backend type", bt.Type)
	}
}

func run(sm *subnet.SubnetManager, be backend.Backend, exit chan int) {
	var err error
	defer func() {
		if err == nil || err == task.ErrCanceled {
			exit <- 0
		} else {
			log.Error(err)
			exit <- 1
		}
	}()

	iface, ipaddr, err := lookupIface()
	if err != nil {
		return
	}

	if iface.MTU == 0 {
		err = fmt.Errorf("Failed to determine MTU for %s interface", ipaddr)
		return
	}

	log.Infof("Using %s as external interface", ipaddr)

	sn, err := be.Init(iface, ipaddr)
	if err != nil {
		return
	}

	if opts.ipMasq {
		flannelNet := sm.GetConfig().Network
		if err = setupIPMasq(flannelNet); err != nil {
			return
		}
	}

	writeSubnetFile(sn)
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

	flagsFromEnv("FLANNELD", flag.CommandLine)

	sm := newSubnetManager()
	be, err := newBackend(sm)
	if err != nil {
		log.Info(err)
		os.Exit(1)
	}

	// Register for SIGINT and SIGTERM and wait for one of them to arrive
	log.Info("Installing signal handlers")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	exit := make(chan int)
	go run(sm, be, exit)

	for {
		select {
		case <-sigs:
			// unregister to get default OS nuke behaviour in case we don't exit cleanly
			signal.Stop(sigs)

			log.Info("Exiting...")
			be.Stop()

		case code := <-exit:
			log.Infof("%s mode exited", be.Name())
			os.Exit(code)
		}
	}
}
