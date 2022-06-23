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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/pkg/flagutil"
	"github.com/flannel-io/flannel/network"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/ipmatch"
	"github.com/flannel-io/flannel/subnet"
	etcd "github.com/flannel-io/flannel/subnet/etcd"
	"github.com/flannel-io/flannel/subnet/kube"
	"github.com/flannel-io/flannel/version"
	"golang.org/x/net/context"
	log "k8s.io/klog"

	"github.com/joho/godotenv"

	// Backends need to be imported for their init() to get executed and them to register
	"github.com/coreos/go-systemd/daemon"
	"github.com/flannel-io/flannel/backend"
	_ "github.com/flannel-io/flannel/backend/alivpc"
	_ "github.com/flannel-io/flannel/backend/alloc"
	_ "github.com/flannel-io/flannel/backend/awsvpc"
	_ "github.com/flannel-io/flannel/backend/extension"
	_ "github.com/flannel-io/flannel/backend/gce"
	_ "github.com/flannel-io/flannel/backend/hostgw"
	_ "github.com/flannel-io/flannel/backend/ipip"
	_ "github.com/flannel-io/flannel/backend/ipsec"
	_ "github.com/flannel-io/flannel/backend/tencentvpc"
	_ "github.com/flannel-io/flannel/backend/udp"
	_ "github.com/flannel-io/flannel/backend/vxlan"
	_ "github.com/flannel-io/flannel/backend/wireguard"
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
		return kube.NewSubnetManager(ctx, opts.kubeApiUrl, opts.kubeConfigFile, opts.kubeAnnotationPrefix, opts.netConfPath, opts.setNodeNetworkUnavailable)
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

	return etcd.NewLocalManager(ctx, cfg, prevSubnet, prevIPv6Subnet)
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

	// Work out which interface to use
	var extIface *backend.ExternalInterface
	optsPublicIP := ipmatch.PublicIPOpts{
		PublicIP:   opts.publicIP,
		PublicIPv6: opts.publicIPv6,
	}
	// Check the default interface only if no interfaces are specified
	if len(opts.iface) == 0 && len(opts.ifaceRegex) == 0 && len(opts.ifaceCanReach) == 0 {
		extIface, err = ipmatch.LookupExtIface(opts.publicIP, "", "", ipStack, optsPublicIP)
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

	// Set up ipMasq if needed
	if opts.ipMasq {
		if config.EnableIPv4 {
			if err = recycleIPTables(config.Network, bn.Lease()); err != nil {
				log.Errorf("Failed to recycle IPTables rules, %v", err)
				cancel()
				wg.Wait()
				os.Exit(1)
			}
			log.Infof("Setting up masking rules")
			go network.SetupAndEnsureIP4Tables(network.MasqRules(config.Network, bn.Lease()), opts.iptablesResyncSeconds)

		}
		if config.EnableIPv6 {
			if err = recycleIP6Tables(config.IPv6Network, bn.Lease()); err != nil {
				log.Errorf("Failed to recycle IP6Tables rules, %v", err)
				cancel()
				wg.Wait()
				os.Exit(1)
			}
			log.Infof("Setting up masking ip6 rules")
			go network.SetupAndEnsureIP6Tables(network.MasqIP6Rules(config.IPv6Network, bn.Lease()), opts.iptablesResyncSeconds)
		}
	}

	// Always enables forwarding rules. This is needed for Docker versions >1.13 (https://docs.docker.com/engine/userguide/networking/default_network/container-communication/#container-communication-between-hosts)
	// In Docker 1.12 and earlier, the default FORWARD chain policy was ACCEPT.
	// In Docker 1.13 and later, Docker sets the default policy of the FORWARD chain to DROP.
	if opts.iptablesForwardRules {
		if config.EnableIPv4 {
			log.Infof("Changing default FORWARD chain policy to ACCEPT")
			go network.SetupAndEnsureIP4Tables(network.ForwardRules(config.Network.String()), opts.iptablesResyncSeconds)
		}
		if config.EnableIPv6 {
			log.Infof("IPv6: Changing default FORWARD chain policy to ACCEPT")
			go network.SetupAndEnsureIP6Tables(network.ForwardRules(config.IPv6Network.String()), opts.iptablesResyncSeconds)
		}
	}

	if err := WriteSubnetFile(opts.subnetFile, config, opts.ipMasq, bn); err != nil {
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

	// Kube subnet mgr doesn't lease the subnet for this node - it just uses the podCidr that's already assigned.
	if !opts.kubeSubnetMgr {
		err = MonitorLease(ctx, sm, bn, &wg)
		if err == errInterrupted {
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

func recycleIPTables(nw ip.IP4Net, lease *subnet.Lease) error {
	prevNetwork := ReadCIDRFromSubnetFile(opts.subnetFile, "FLANNEL_NETWORK")
	prevSubnet := ReadCIDRFromSubnetFile(opts.subnetFile, "FLANNEL_SUBNET")
	// recycle iptables rules only when network configured or subnet leased is not equal to current one.
	if prevNetwork != nw && prevSubnet != lease.Subnet {
		log.Infof("Current network or subnet (%v, %v) is not equal to previous one (%v, %v), trying to recycle old iptables rules", nw, lease.Subnet, prevNetwork, prevSubnet)
		lease := &subnet.Lease{
			Subnet: prevSubnet,
		}
		if err := network.DeleteIP4Tables(network.MasqRules(prevNetwork, lease)); err != nil {
			return err
		}
	}
	return nil
}

func recycleIP6Tables(nw ip.IP6Net, lease *subnet.Lease) error {
	prevNetwork := ReadIP6CIDRFromSubnetFile(opts.subnetFile, "FLANNEL_IPV6_NETWORK")
	prevSubnet := ReadIP6CIDRFromSubnetFile(opts.subnetFile, "FLANNEL_IPV6_SUBNET")
	// recycle iptables rules only when network configured or subnet leased is not equal to current one.
	if prevNetwork.String() != nw.String() && prevSubnet.String() != lease.IPv6Subnet.String() {
		log.Infof("Current ipv6 network or subnet (%v, %v) is not equal to previous one (%v, %v), trying to recycle old ip6tables rules", nw, lease.IPv6Subnet, prevNetwork, prevSubnet)
		lease := &subnet.Lease{
			IPv6Subnet: prevSubnet,
		}
		if err := network.DeleteIP6Tables(network.MasqIP6Rules(prevNetwork, lease)); err != nil {
			return err
		}
	}
	return nil
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

func MonitorLease(ctx context.Context, sm subnet.Manager, bn backend.Network, wg *sync.WaitGroup) error {
	// Use the subnet manager to start watching leases.
	evts := make(chan subnet.Event)

	wg.Add(1)
	go func() {
		l := bn.Lease()
		subnet.WatchLease(ctx, sm, l.Subnet, l.IPv6Subnet, evts)
		wg.Done()
	}()

	renewMargin := time.Duration(opts.subnetLeaseRenewMargin) * time.Minute
	dur := time.Until(bn.Lease().Expiration) - renewMargin

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
			dur = time.Until(bn.Lease().Expiration) - renewMargin

		case e, ok := <-evts:
			if !ok {
				log.Infof("Stopped monitoring lease")
				return errCanceled
			}
			switch e.Type {
			case subnet.EventAdded:
				bn.Lease().Expiration = e.Lease.Expiration
				dur = time.Until(bn.Lease().Expiration) - renewMargin
				log.Infof("Waiting for %s to renew lease", dur)

			case subnet.EventRemoved:
				log.Error("Lease has been revoked. Shutting down daemon.")
				return errInterrupted
			}
		}
	}
}

func WriteSubnetFile(path string, config *subnet.Config, ipMasq bool, bn backend.Network) error {
	dir, name := filepath.Split(path)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	tempFile := filepath.Join(dir, "."+name)
	f, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	if config.EnableIPv4 {
		nw := config.Network
		sn := bn.Lease().Subnet
		// Write out the first usable IP by incrementing sn.IP by one
		sn.IncrementIP()
		fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", nw)
		fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn)
	}
	if config.EnableIPv6 {
		ip6Nw := config.IPv6Network
		ip6Sn := bn.Lease().IPv6Subnet
		// Write out the first usable IP by incrementing ip6Sn.IP by one
		ip6Sn.IncrementIP()
		fmt.Fprintf(f, "FLANNEL_IPV6_NETWORK=%s\n", ip6Nw)
		fmt.Fprintf(f, "FLANNEL_IPV6_SUBNET=%s\n", ip6Sn)
	}

	fmt.Fprintf(f, "FLANNEL_MTU=%d\n", bn.MTU())
	_, err = fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq)
	f.Close()
	if err != nil {
		return err
	}

	// rename(2) the temporary file to the desired location so that it becomes
	// atomically visible with the contents
	return os.Rename(tempFile, path)
	// TODO - is this safe? What if it's not on the same FS?
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
		ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Shutdown healthz server error. %v", err)
		}
		wg.Done()
	}()
}

func ReadCIDRFromSubnetFile(path string, CIDRKey string) ip.IP4Net {
	var prevCIDR ip.IP4Net
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		prevSubnetVals, err := godotenv.Read(path)
		if err != nil {
			log.Errorf("Couldn't fetch previous %s from subnet file at %s: %s", CIDRKey, path, err)
		} else if prevCIDRString, ok := prevSubnetVals[CIDRKey]; ok {
			err = prevCIDR.UnmarshalJSON([]byte(prevCIDRString))
			if err != nil {
				log.Errorf("Couldn't parse previous %s from subnet file at %s: %s", CIDRKey, path, err)
			}
		}
	}
	return prevCIDR
}

func ReadIP6CIDRFromSubnetFile(path string, CIDRKey string) ip.IP6Net {
	var prevCIDR ip.IP6Net
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		prevSubnetVals, err := godotenv.Read(path)
		if err != nil {
			log.Errorf("Couldn't fetch previous %s from subnet file at %s: %s", CIDRKey, path, err)
		} else if prevCIDRString, ok := prevSubnetVals[CIDRKey]; ok {
			err = prevCIDR.UnmarshalJSON([]byte(prevCIDRString))
			if err != nil {
				log.Errorf("Couldn't parse previous %s from subnet file at %s: %s", CIDRKey, path, err)
			}
		}
	}
	return prevCIDR
}
