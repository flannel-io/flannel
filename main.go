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
	"context"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/pkg/flagutil"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/ipmatch"
	"github.com/flannel-io/flannel/pkg/subnet"
	etcd "github.com/flannel-io/flannel/pkg/subnet/etcd"
	"github.com/flannel-io/flannel/pkg/subnet/kube"
	"github.com/flannel-io/flannel/pkg/trafficmngr"
	"github.com/flannel-io/flannel/pkg/trafficmngr/iptables"
	"github.com/flannel-io/flannel/pkg/trafficmngr/nftables"
	"github.com/flannel-io/flannel/pkg/version"
	"github.com/joho/godotenv"
	log "k8s.io/klog/v2"

	// Backends need to be imported for their init() to get executed and them to register
	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/flannel-io/flannel/pkg/backend"
	_ "github.com/flannel-io/flannel/pkg/backend/alloc"
	_ "github.com/flannel-io/flannel/pkg/backend/extension"
	_ "github.com/flannel-io/flannel/pkg/backend/hostgw"
	_ "github.com/flannel-io/flannel/pkg/backend/ipip"
	_ "github.com/flannel-io/flannel/pkg/backend/ipsec"
	_ "github.com/flannel-io/flannel/pkg/backend/tencentvpc"
	_ "github.com/flannel-io/flannel/pkg/backend/udp"
	_ "github.com/flannel-io/flannel/pkg/backend/vxlan"
	_ "github.com/flannel-io/flannel/pkg/backend/wireguard"
)

type flagSlice []string

func (t *flagSlice) String() string {
	return fmt.Sprintf("%v", *t)
}

func (t *flagSlice) Set(val string) error {
	*t = append(*t, val)
	return nil
}

type CmdLineOpts struct {
	etcdEndpoints             string
	etcdPrefix                string
	etcdKeyfile               string
	etcdCertfile              string
	etcdCAFile                string
	etcdUsername              string
	etcdPassword              string
	version                   bool
	kubeSubnetMgr             bool
	kubeApiUrl                string
	kubeAnnotationPrefix      string
	kubeConfigFile            string
	iface                     flagSlice
	ifaceRegex                flagSlice
	ipMasq                    bool
	ifaceCanReach             string
	subnetFile                string
	publicIP                  string
	publicIPv6                string
	subnetLeaseRenewMargin    int
	healthzIP                 string
	healthzPort               int
	iptablesResyncSeconds     int
	iptablesForwardRules      bool
	netConfPath               string
	setNodeNetworkUnavailable bool
}

var (
	opts           CmdLineOpts
	errInterrupted = errors.New("interrupted")
	errCanceled    = errors.New("canceled")
	flannelFlags   = flag.NewFlagSet("flannel", flag.ExitOnError)
)

func init() {
	flannelFlags.StringVar(&opts.etcdEndpoints, "etcd-endpoints", "http://127.0.0.1:4001,http://127.0.0.1:2379", "a comma-delimited list of etcd endpoints")
	flannelFlags.StringVar(&opts.etcdPrefix, "etcd-prefix", "/coreos.com/network", "etcd prefix")
	flannelFlags.StringVar(&opts.etcdKeyfile, "etcd-keyfile", "", "SSL key file used to secure etcd communication")
	flannelFlags.StringVar(&opts.etcdCertfile, "etcd-certfile", "", "SSL certification file used to secure etcd communication")
	flannelFlags.StringVar(&opts.etcdCAFile, "etcd-cafile", "", "SSL Certificate Authority file used to secure etcd communication")
	flannelFlags.StringVar(&opts.etcdUsername, "etcd-username", "", "username for BasicAuth to etcd")
	flannelFlags.StringVar(&opts.etcdPassword, "etcd-password", "", "password for BasicAuth to etcd")
	flannelFlags.Var(&opts.iface, "iface", "interface to use (IP or name) for inter-host communication. Can be specified multiple times to check each option in order. Returns the first match found.")
	flannelFlags.Var(&opts.ifaceRegex, "iface-regex", "regex expression to match the first interface to use (IP or name) for inter-host communication. Can be specified multiple times to check each regex in order. Returns the first match found. Regexes are checked after specific interfaces specified by the iface option have already been checked.")
	flannelFlags.StringVar(&opts.ifaceCanReach, "iface-can-reach", "", "detect interface to use (IP or name) for inter-host communication based on which will be used for provided IP. This is exactly the interface to use of command 'ip route get <ip-address>'")
	flannelFlags.StringVar(&opts.subnetFile, "subnet-file", "/run/flannel/subnet.env", "filename where env variables (subnet, MTU, ... ) will be written to")
	flannelFlags.StringVar(&opts.publicIP, "public-ip", "", "IP accessible by other nodes for inter-host communication")
	flannelFlags.StringVar(&opts.publicIPv6, "public-ipv6", "", "IPv6 accessible by other nodes for inter-host communication")
	flannelFlags.IntVar(&opts.subnetLeaseRenewMargin, "subnet-lease-renew-margin", 60, "subnet lease renewal margin, in minutes, ranging from 1 to 1439")
	flannelFlags.BoolVar(&opts.ipMasq, "ip-masq", false, "setup IP masquerade rule for traffic destined outside of overlay network")
	flannelFlags.BoolVar(&opts.kubeSubnetMgr, "kube-subnet-mgr", false, "contact the Kubernetes API for subnet assignment instead of etcd.")
	flannelFlags.StringVar(&opts.kubeApiUrl, "kube-api-url", "", "Kubernetes API server URL. Does not need to be specified if flannel is running in a pod.")
	flannelFlags.StringVar(&opts.kubeAnnotationPrefix, "kube-annotation-prefix", "flannel.alpha.coreos.com", `Kubernetes annotation prefix. Can contain single slash "/", otherwise it will be appended at the end.`)
	flannelFlags.StringVar(&opts.kubeConfigFile, "kubeconfig-file", "", "kubeconfig file location. Does not need to be specified if flannel is running in a pod.")
	flannelFlags.BoolVar(&opts.version, "version", false, "print version and exit")
	flannelFlags.StringVar(&opts.healthzIP, "healthz-ip", "0.0.0.0", "the IP address for healthz server to listen")
	flannelFlags.IntVar(&opts.healthzPort, "healthz-port", 0, "the port for healthz server to listen(0 to disable)")
	flannelFlags.IntVar(&opts.iptablesResyncSeconds, "iptables-resync", 5, "resync period for iptables rules, in seconds")
	flannelFlags.BoolVar(&opts.iptablesForwardRules, "iptables-forward-rules", true, "add default accept rules to FORWARD chain in iptables")
	flannelFlags.StringVar(&opts.netConfPath, "net-config-path", "/etc/kube-flannel/net-conf.json", "path to the network configuration file")
	flannelFlags.BoolVar(&opts.setNodeNetworkUnavailable, "set-node-network-unavailable", true, "set NodeNetworkUnavailable after ready")

	log.InitFlags(nil)

	// klog will log to tmp files by default. override so all entries
	// can flow into journald (if running under systemd)
	err := flag.Set("logtostderr", "true")
	if err != nil {
		log.Error("Can't set the logtostderr flag", err)
		os.Exit(1)
	}

	// Only copy the non file logging options from klog
	copyFlag("v")
	copyFlag("vmodule")
	copyFlag("log_backtrace_at")

	// Define the usage function
	flannelFlags.Usage = usage

	// now parse command line args
	err = flannelFlags.Parse(os.Args[1:])
	if err != nil {
		log.Error("Can't parse flannel flags", err)
		os.Exit(1)
	}
}

func copyFlag(name string) {
	flannelFlags.Var(flag.Lookup(name).Value, flag.Lookup(name).Name, flag.Lookup(name).Usage)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
	flannelFlags.PrintDefaults()
	os.Exit(0)
}

func newSubnetManager(ctx context.Context) (subnet.Manager, error) {
	if opts.kubeSubnetMgr {
		return kube.NewSubnetManager(ctx,
			opts.kubeApiUrl,
			opts.kubeConfigFile,
			opts.kubeAnnotationPrefix,
			opts.netConfPath,
			opts.setNodeNetworkUnavailable)
	}

	cfg := &etcd.EtcdConfig{
		Endpoints: strings.Split(opts.etcdEndpoints, ","),
		Keyfile:   opts.etcdKeyfile,
		Certfile:  opts.etcdCertfile,
		CAFile:    opts.etcdCAFile,
		Prefix:    opts.etcdPrefix,
		Username:  opts.etcdUsername,
		Password:  opts.etcdPassword,
	}

	// Attempt to renew the lease for the subnet specified in the subnetFile
	prevSubnet := ReadCIDRFromSubnetFile(opts.subnetFile, "FLANNEL_SUBNET")
	prevIPv6Subnet := ReadIP6CIDRFromSubnetFile(opts.subnetFile, "FLANNEL_IPV6_SUBNET")

	return etcd.NewLocalManager(ctx, cfg, prevSubnet, prevIPv6Subnet, opts.subnetLeaseRenewMargin)
}

func main() {
	if opts.version {
		fmt.Fprintln(os.Stderr, version.Version)
		os.Exit(0)
	}

	err := flagutil.SetFlagsFromEnv(flannelFlags, "FLANNELD")
	if err != nil {
		log.Error("Failed to set flag FLANNELD from env", err)
	}

	// Log the config set via CLI flags
	log.Infof("CLI flags config: %+v", opts)

	// Validate flags
	if opts.subnetLeaseRenewMargin >= 24*60 || opts.subnetLeaseRenewMargin <= 0 {
		log.Error("Invalid subnet-lease-renew-margin option, out of acceptable range")
		os.Exit(1)
	}

	// This is the main context that everything should run in.
	// All spawned goroutines should exit when cancel is called on this context.
	// Go routines spawned from main.go coordinate using a WaitGroup. This provides a mechanism to allow the shutdownHandler goroutine
	// to block until all the goroutines return . If those goroutines spawn other goroutines then they are responsible for
	// blocking and returning only when cancel() is called.
	ctx, cancel := context.WithCancel(context.Background())

	sm, err := newSubnetManager(ctx)
	if err != nil {
		log.Error("Failed to create SubnetManager: ", err)
		os.Exit(1)
	}
	log.Infof("Created subnet manager: %s", sm.Name())

	// Register for SIGINT and SIGTERM
	log.Info("Installing signal handlers")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		shutdownHandler(ctx, sigs, cancel)
		wg.Done()
	}()

	if opts.healthzPort > 0 {
		mustRunHealthz(ctx.Done(), &wg)
	}

	// Fetch the network config (i.e. what backend to use etc..).
	config, err := getConfig(ctx, sm)
	if err == errCanceled {
		wg.Wait()
		os.Exit(0)
	}

	// Get ip family stack
	ipStack, stackErr := ipmatch.GetIPFamily(config.EnableIPv4, config.EnableIPv6)
	if stackErr != nil {
		log.Error(stackErr.Error())
		os.Exit(1)
	}

	if runtime.GOOS != "windows" {
		// From Kubernetes 1.30 kubeadm doesn't check if the br_netfilter module is loaded and in case it's missing Flannel wrongly starts
		if config.EnableIPv4 {
			if _, err = os.Stat("/proc/sys/net/bridge/bridge-nf-call-iptables"); os.IsNotExist(err) {
				log.Error("Failed to check br_netfilter: ", err)
				os.Exit(1)
			}
		}
		if config.EnableIPv6 {
			if _, err = os.Stat("/proc/sys/net/bridge/bridge-nf-call-ip6tables"); os.IsNotExist(err) {
				log.Error("Failed to check br_netfilter: ", err)
				os.Exit(1)
			}
		}
	}

	// Work out which interface to use
	var extIface *backend.ExternalInterface

	annotatedPublicIP, annotatedPublicIPv6 := sm.GetStoredPublicIP(ctx)
	if annotatedPublicIP != "" {
		opts.publicIP = annotatedPublicIP
	}
	if annotatedPublicIPv6 != "" {
		opts.publicIPv6 = annotatedPublicIPv6
	}

	optsPublicIP := ipmatch.PublicIPOpts{
		PublicIP:   opts.publicIP,
		PublicIPv6: opts.publicIPv6,
	}
	// Check the default interface only if no interfaces are specified
	if len(opts.iface) == 0 && len(opts.ifaceRegex) == 0 && len(opts.ifaceCanReach) == 0 {
		if len(opts.publicIP) > 0 {
			extIface, err = ipmatch.LookupExtIface(opts.publicIP, "", "", ipStack, optsPublicIP)
		} else {
			extIface, err = ipmatch.LookupExtIface(opts.publicIPv6, "", "", ipStack, optsPublicIP)
		}
		if err != nil {
			log.Error("Failed to find any valid interface to use: ", err)
			os.Exit(1)
		}
	} else {
		// Check explicitly specified interfaces
		for _, iface := range opts.iface {
			extIface, err = ipmatch.LookupExtIface(iface, "", "", ipStack, optsPublicIP)
			if err != nil {
				log.Infof("Could not find valid interface matching %s: %s", iface, err)
			}

			if extIface != nil {
				break
			}
		}

		// Check interfaces that match any specified regexes
		if extIface == nil {
			for _, ifaceRegex := range opts.ifaceRegex {
				extIface, err = ipmatch.LookupExtIface("", ifaceRegex, "", ipStack, optsPublicIP)
				if err != nil {
					log.Infof("Could not find valid interface matching %s: %s", ifaceRegex, err)
				}

				if extIface != nil {
					break
				}
			}
		}

		if extIface == nil && len(opts.ifaceCanReach) > 0 {
			extIface, err = ipmatch.LookupExtIface("", "", opts.ifaceCanReach, ipStack, optsPublicIP)
			if err != nil {
				log.Infof("Could not find valid interface matching ifaceCanReach: %s: %s", opts.ifaceCanReach, err)
			}
		}

		if extIface == nil {
			// Exit if any of the specified interfaces do not match
			log.Error("Failed to find interface to use that matches the interfaces and/or regexes provided")
			os.Exit(1)
		}
	}

	// Create a backend manager then use it to create the backend and register the network with it.
	bm := backend.NewManager(ctx, sm, extIface)
	be, err := bm.GetBackend(config.BackendType)
	if err != nil {
		log.Errorf("Error fetching backend: %s", err)
		cancel()
		wg.Wait()
		os.Exit(1)
	}

	bn, err := be.RegisterNetwork(ctx, &wg, config)
	if err != nil {
		log.Errorf("Error registering network: %s", err)
		cancel()
		wg.Wait()
		os.Exit(1)
	}

	//Create TrafficManager and instantiate it based on whether we use iptables or nftables
	trafficMngr := newTrafficManager(config.EnableNFTables)
	err = trafficMngr.Init(ctx, &wg)
	if err != nil {
		log.Error(err)
		cancel()
		wg.Wait()
		os.Exit(1)
	}

	// Set up ipMasq if needed
	if opts.ipMasq {
		prevNetwork := ReadCIDRFromSubnetFile(opts.subnetFile, "FLANNEL_NETWORK")
		prevSubnet := ReadCIDRFromSubnetFile(opts.subnetFile, "FLANNEL_SUBNET")

		prevIPv6Network := ReadIP6CIDRFromSubnetFile(opts.subnetFile, "FLANNEL_IPV6_NETWORK")
		prevIPv6Subnet := ReadIP6CIDRFromSubnetFile(opts.subnetFile, "FLANNEL_IPV6_SUBNET")

		err = trafficMngr.SetupAndEnsureMasqRules(ctx,
			config.Network, prevSubnet,
			prevNetwork,
			config.IPv6Network, prevIPv6Subnet,
			prevIPv6Network,
			bn.Lease(),
			opts.iptablesResyncSeconds)
		if err != nil {
			log.Errorf("Failed to setup masq rules, %v", err)
			cancel()
			wg.Wait()
			os.Exit(1)
		}
	}

	// Always enables forwarding rules. This is needed for Docker versions >1.13 (https://docs.docker.com/engine/userguide/networking/default_network/container-communication/#container-communication-between-hosts)
	// In Docker 1.12 and earlier, the default FORWARD chain policy was ACCEPT.
	// In Docker 1.13 and later, Docker sets the default policy of the FORWARD chain to DROP.
	if opts.iptablesForwardRules {
		trafficMngr.SetupAndEnsureForwardRules(ctx,
			config.Network,
			config.IPv6Network,
			opts.iptablesResyncSeconds)
	}

	if err := sm.HandleSubnetFile(opts.subnetFile, config, opts.ipMasq, bn.Lease().Subnet, bn.Lease().IPv6Subnet, bn.MTU()); err != nil {
		// Continue, even though it failed.
		log.Warningf("Failed to write subnet file: %s", err)
	} else {
		log.Infof("Wrote subnet file to %s", opts.subnetFile)
	}

	// Start "Running" the backend network. This will block until the context is done so run in another goroutine.
	log.Info("Running backend.")
	wg.Add(1)
	go func() {
		bn.Run(ctx)
		wg.Done()
	}()

	_, err = daemon.SdNotify(false, "READY=1")
	if err != nil {
		log.Errorf("Failed to notify systemd the message READY=1 %v", err)
	}

	err = sm.CompleteLease(ctx, bn.Lease(), &wg)
	if err != nil {
		log.Errorf("CompleteLease execute error err: %v", err)
		if strings.EqualFold(err.Error(), errInterrupted.Error()) {
			// The lease was "revoked" - shut everything down
			cancel()
		}
	}

	log.Info("Waiting for all goroutines to exit")
	// Block waiting for all the goroutines to finish.
	wg.Wait()
	log.Info("Exiting cleanly...")
	os.Exit(0)
}

func shutdownHandler(ctx context.Context, sigs chan os.Signal, cancel context.CancelFunc) {
	// Wait for the context do be Done or for the signal to come in to shutdown.
	select {
	case <-ctx.Done():
		log.Info("Stopping shutdownHandler...")
	case <-sigs:
		// Call cancel on the context to close everything down.
		cancel()
		log.Info("shutdownHandler sent cancel signal...")
	}

	// Unregister to get default OS nuke behaviour in case we don't exit cleanly
	signal.Stop(sigs)
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

func mustRunHealthz(stopChan <-chan struct{}, wg *sync.WaitGroup) {
	address := net.JoinHostPort(opts.healthzIP, strconv.Itoa(opts.healthzPort))
	log.Infof("Start healthz server on %s", address)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("flanneld is running"))
		if err != nil {
			log.Errorf("Handling /healthz error. %v", err)
			panic(err)
		}
	})

	server := &http.Server{Addr: address}

	wg.Add(2)
	go func() {
		// when Shutdown is called, ListenAndServe immediately return ErrServerClosed.
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Errorf("Start healthz server error. %v", err)
			panic(err)
		}
		wg.Done()
	}()

	go func() {
		// wait to stop
		<-stopChan

		// create new context with timeout for http server to shutdown gracefully
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Shutdown healthz server error. %v", err)
		}
		wg.Done()
	}()
}

func ReadCIDRFromSubnetFile(path string, CIDRKey string) ip.IP4Net {
	prevCIDRs := ReadCIDRsFromSubnetFile(path, CIDRKey)
	if len(prevCIDRs) == 0 {
		log.Warningf("no subnet found for key: %s in file: %s", CIDRKey, path)
		return ip.IP4Net{IP: 0, PrefixLen: 0}
	} else if len(prevCIDRs) > 1 {
		log.Errorf("error reading subnet: more than 1 entry found for key: %s in file %s: ", CIDRKey, path)
		return ip.IP4Net{IP: 0, PrefixLen: 0}
	} else {
		return prevCIDRs[0]
	}
}

func ReadCIDRsFromSubnetFile(path string, CIDRKey string) []ip.IP4Net {
	prevCIDRs := make([]ip.IP4Net, 0)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		prevSubnetVals, err := godotenv.Read(path)
		if err != nil {
			log.Errorf("Couldn't fetch previous %s from subnet file at %s: %s", CIDRKey, path, err)
		} else if prevCIDRString, ok := prevSubnetVals[CIDRKey]; ok {
			cidrs := strings.Split(prevCIDRString, ",")
			prevCIDRs = make([]ip.IP4Net, 0)
			for i := range cidrs {
				_, cidr, err := net.ParseCIDR(cidrs[i])
				if err != nil {
					log.Errorf("Couldn't parse previous %s from subnet file at %s: %s", CIDRKey, path, err)
				}
				prevCIDRs = append(prevCIDRs, ip.FromIPNet(cidr))
			}

		}
	}
	return prevCIDRs
}

func ReadIP6CIDRFromSubnetFile(path string, CIDRKey string) ip.IP6Net {
	prevCIDRs := ReadIP6CIDRsFromSubnetFile(path, CIDRKey)
	if len(prevCIDRs) == 0 {
		log.Warningf("no subnet found for key: %s in file: %s", CIDRKey, path)
		return ip.IP6Net{IP: (*ip.IP6)(big.NewInt(0)), PrefixLen: 0}
	} else if len(prevCIDRs) > 1 {
		log.Errorf("error reading subnet: more than 1 entry found for key: %s in file %s: ", CIDRKey, path)
		return ip.IP6Net{IP: (*ip.IP6)(big.NewInt(0)), PrefixLen: 0}
	} else {
		return prevCIDRs[0]
	}
}

func ReadIP6CIDRsFromSubnetFile(path string, CIDRKey string) []ip.IP6Net {
	prevCIDRs := make([]ip.IP6Net, 0)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		prevSubnetVals, err := godotenv.Read(path)
		if err != nil {
			log.Errorf("Couldn't fetch previous %s from subnet file at %s: %s", CIDRKey, path, err)
		} else if prevCIDRString, ok := prevSubnetVals[CIDRKey]; ok {
			cidrs := strings.Split(prevCIDRString, ",")
			prevCIDRs = make([]ip.IP6Net, 0)
			for i := range cidrs {
				_, cidr, err := net.ParseCIDR(cidrs[i])
				if err != nil {
					log.Errorf("Couldn't parse previous %s from subnet file at %s: %s", CIDRKey, path, err)
				}
				prevCIDRs = append(prevCIDRs, ip.FromIP6Net(cidr))
			}

		}
	}
	return prevCIDRs
}

func newTrafficManager(useNftables bool) trafficmngr.TrafficManager {
	if useNftables {
		return &nftables.NFTablesManager{}
	} else {
		return &iptables.IPTablesManager{}
	}
}
