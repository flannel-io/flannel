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

package etcd

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/subnet"
	. "github.com/flannel-io/flannel/pkg/subnet"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/tlsutil"
	etcd "go.etcd.io/etcd/client/v3"
	"golang.org/x/net/context"
	log "k8s.io/klog"
)

var (
	errTryAgain            = errors.New("try again")
	errConfigNotFound      = errors.New("flannel config not found in etcd store. Did you create your config using etcdv3 API?")
	errNoWatchChannel      = errors.New("no watch channel")
	errSubnetAlreadyexists = errors.New("subnet already exists")
)

type Registry interface {
	getNetworkConfig(ctx context.Context) (string, error)
	getSubnets(ctx context.Context) ([]Lease, int64, error)
	getSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) (*Lease, int64, error)
	createSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *LeaseAttrs, ttl time.Duration) (time.Time, error)
	updateSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *LeaseAttrs, ttl time.Duration, asof int64) (time.Time, error)
	deleteSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) error
	watchSubnets(ctx context.Context, leaseWatchChan chan []LeaseWatchResult, since int64) error
	watchSubnet(ctx context.Context, since int64, sn ip.IP4Net, sn6 ip.IP6Net, leaseWatchChan chan []LeaseWatchResult) error
	leasesWatchReset(ctx context.Context) (LeaseWatchResult, error)
}

type EtcdConfig struct {
	Endpoints []string
	Keyfile   string
	Certfile  string
	CAFile    string
	Prefix    string
	Username  string
	Password  string
}

type etcdNewFunc func(ctx context.Context, c *EtcdConfig) (*etcd.Client, etcd.KV, error)

type etcdSubnetRegistry struct {
	cliNewFunc   etcdNewFunc
	mux          sync.Mutex
	kvApi        etcd.KV
	cli          *etcd.Client
	etcdCfg      *EtcdConfig
	networkRegex *regexp.Regexp
}

func newTlsConfig(c *EtcdConfig) (*tls.Config, error) {
	tlscfg := tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if c.Keyfile == "" || c.Certfile == "" {
		log.Warning("no certificate provided: connecting to etcd with http. This is insecure")
		return nil, nil
	} else {
		cert, err := tlsutil.NewCert(c.Certfile, c.Keyfile, nil)
		if err != nil {
			return nil, err
		}

		if cert != nil {
			tlscfg.Certificates = []tls.Certificate{*cert}
		}
		if c.CAFile != "" {
			tlscfg.RootCAs, err = tlsutil.NewCertPool([]string{c.CAFile})
			if err != nil {
				return nil, err
			}
		}
	}

	return &tlscfg, nil
}

func newEtcdClient(ctx context.Context, c *EtcdConfig) (*etcd.Client, etcd.KV, error) {
	tlscfg, err := newTlsConfig(c)
	if err != nil {
		return nil, nil, err
	}

	cli, err := etcd.New(etcd.Config{
		Endpoints: c.Endpoints,
		Username:  c.Username,
		Password:  c.Password,
		TLS:       tlscfg,
	})
	if err != nil {
		return nil, nil, err
	}
	kv := etcd.NewKV(cli)

	//make sure the Client is closed properly
	go func() {
		<-ctx.Done()
		cli.Close()
	}()
	return cli, kv, nil
}

func newEtcdSubnetRegistry(ctx context.Context, config *EtcdConfig, cliNewFunc etcdNewFunc) (Registry, error) {
	r := &etcdSubnetRegistry{
		etcdCfg:      config,
		networkRegex: regexp.MustCompile(config.Prefix + `/([^/]*)(/|/config)?$`),
	}
	if cliNewFunc != nil {
		r.cliNewFunc = cliNewFunc
	} else {
		r.cliNewFunc = newEtcdClient
	}

	var err error
	r.cli, r.kvApi, err = r.cliNewFunc(ctx, config)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (esr *etcdSubnetRegistry) getNetworkConfig(ctx context.Context) (string, error) {
	key := path.Join(esr.etcdCfg.Prefix, "config")
	resp, err := esr.kv().Get(ctx, key)

	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", errConfigNotFound
	}

	return string(resp.Kvs[0].Value), nil
}

// getSubnets queries etcd to get a list of currently allocated leases for a given network.
// It returns the leases along with the "as-of" etcd-index that can be used as the starting
// point for etcd watch.
func (esr *etcdSubnetRegistry) getSubnets(ctx context.Context) ([]Lease, int64, error) {
	key := path.Join(esr.etcdCfg.Prefix, "subnets")
	resp, err := esr.kv().Get(ctx, key, etcd.WithPrefix())
	if err != nil {
		if err == rpctypes.ErrGRPCKeyNotFound {
			// key not found: treat it as empty set
			return []Lease{}, 0, nil
		}
		return nil, 0, err
	}

	leases := []Lease{}
	for _, kv := range resp.Kvs {
		ttlresp, err := esr.cli.TimeToLive(ctx, etcd.LeaseID(kv.Lease))
		if err != nil {
			log.Warningf("Could not read ttl: %v", err)
			continue
		}
		l, err := kvToIPLease(kv, ttlresp.TTL)
		if err != nil {
			log.Warningf("Ignoring bad subnet node: %v", err)
			continue
		}

		leases = append(leases, *l)
	}

	return leases, resp.Header.Revision, nil
}

func (esr *etcdSubnetRegistry) getSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) (*Lease, int64, error) {
	key := path.Join(esr.etcdCfg.Prefix, "subnets", MakeSubnetKey(sn, sn6))
	resp, err := esr.kv().Get(ctx, key)
	if err != nil {
		return nil, 0, err
	}

	if len(resp.Kvs) == 0 {
		return nil, 0, rpctypes.ErrGRPCKeyNotFound
	}

	ttlresp, err := esr.cli.TimeToLive(ctx, etcd.LeaseID(resp.Kvs[0].Lease))
	if err != nil {
		return nil, 0, err
	}
	l, err := kvToIPLease(resp.Kvs[0], ttlresp.TTL)
	return l, resp.Header.Revision, err
}

func (esr *etcdSubnetRegistry) createSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *LeaseAttrs, ttl time.Duration) (time.Time, error) {
	key := path.Join(esr.etcdCfg.Prefix, "subnets", MakeSubnetKey(sn, sn6))
	value, err := json.Marshal(attrs)
	if err != nil {
		return time.Time{}, err
	}

	lresp, err := esr.cli.Grant(ctx, int64(ttl.Seconds()))
	if err != nil {
		return time.Time{}, err
	}

	//Use a transaction to check if key was not already present in etcd
	req := etcd.OpPut(key, string(value), etcd.WithLease(lresp.ID))
	cond := etcd.Compare(etcd.Version(key), "=", 0)
	tresp, err := esr.cli.Txn(ctx).If(cond).Then(req).Commit()
	if err != nil {
		_, rerr := esr.cli.Revoke(ctx, lresp.ID)
		if rerr != nil {
			log.Error(rerr)
		}
		return time.Time{}, err
	}
	if !tresp.Succeeded {
		_, rerr := esr.cli.Revoke(ctx, lresp.ID)
		if rerr != nil {
			log.Error(rerr)
		}
		return time.Time{}, errSubnetAlreadyexists
	}

	exp := time.Now().Add(time.Duration(lresp.TTL) * time.Second)
	return exp, nil
}

func (esr *etcdSubnetRegistry) updateSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *LeaseAttrs, ttl time.Duration, asof int64) (time.Time, error) {
	key := path.Join(esr.etcdCfg.Prefix, "subnets", MakeSubnetKey(sn, sn6))
	value, err := json.Marshal(attrs)
	if err != nil {
		return time.Time{}, err
	}

	lresp, lerr := esr.cli.Grant(ctx, int64(ttl.Seconds()))
	if lerr != nil {
		return time.Time{}, lerr
	}

	_, perr := esr.kv().Put(ctx, key, string(value), etcd.WithLease(lresp.ID))
	if perr != nil {
		_, rerr := esr.cli.Revoke(ctx, lresp.ID)
		if rerr != nil {
			log.Error(rerr)
		}
		return time.Time{}, perr
	}

	exp := time.Now().Add(time.Duration(lresp.TTL) * time.Second)

	return exp, nil
}

func (esr *etcdSubnetRegistry) deleteSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) error {
	key := path.Join(esr.etcdCfg.Prefix, "subnets", MakeSubnetKey(sn, sn6))
	_, err := esr.kv().Delete(ctx, key)
	return err
}

func (esr *etcdSubnetRegistry) watchSubnets(ctx context.Context, leaseWatchChan chan []LeaseWatchResult, since int64) error {
	key := path.Join(esr.etcdCfg.Prefix, "subnets")

	wctx, cancel := context.WithCancel(ctx)
	//release context ASAP to free resources
	defer cancel()

	log.Infof("registry: watching subnets starting from rev %d", since)
	rch := esr.cli.Watch(etcd.WithRequireLeader(wctx), key, etcd.WithPrefix(), etcd.WithRev(since))
	if rch == nil {
		return errNoWatchChannel
	}
	for {
		select {
		case <-ctx.Done():
			esr.cli.Close()
			close(leaseWatchChan)
			return ctx.Err()
		case wresp := <-rch:
			results := make([]LeaseWatchResult, 0)
			for _, etcdEvent := range wresp.Events {
				subnetEvent, err := parseSubnetWatchResponse(ctx, esr.cli, etcdEvent)
				switch {

				case err == nil:
					log.Infof("watchSubnets: got valid subnet event with revision %d", wresp.Header.Revision)
					// TODO only vxlan backend and kube subnet manager support dual stack now.
					subnetEvent.Lease.EnableIPv4 = true
					wr := subnet.LeaseWatchResult{
						Events: []subnet.Event{subnetEvent},
						Cursor: watchCursor{wresp.Header.Revision},
					}
					results = append(results, wr)

				case isIndexTooSmall(err):
					log.Warning("Watch of subnet leases failed because etcd index outside history window")
					wr, err := esr.leasesWatchReset(ctx)
					if err != nil {
						log.Errorf("error resetting etcd watch: %s", err)
					}
					results = append(results, wr)
				case wresp.Header.Revision != 0:
					log.Warning("Watch of subnet leases failed because header revision != 0")
					results = append(results, LeaseWatchResult{Cursor: watchCursor{wresp.Header.Revision}})

				default:
					log.Warningf("Watch of subnet failed with error %s", err)
					results = append(results, LeaseWatchResult{})
				}
				if err != nil {
					log.Errorf("error parsing etcd event: %s", err)
				}
			}
			if len(results) > 0 {
				leaseWatchChan <- results
			}
		}

	}
}

func (esr *etcdSubnetRegistry) watchSubnet(ctx context.Context, since int64, sn ip.IP4Net, sn6 ip.IP6Net, leaseWatchChan chan []LeaseWatchResult) error {
	key := path.Join(esr.etcdCfg.Prefix, "subnets", MakeSubnetKey(sn, sn6))

	wctx, cancel := context.WithCancel(ctx)
	//release context ASAP to free resources
	defer cancel()

	rch := esr.cli.Watch(etcd.WithRequireLeader(wctx), key, etcd.WithPrefix(), etcd.WithRev(since))
	if rch == nil {
		return errNoWatchChannel
	}

	for {
		select {
		case <-ctx.Done():
			esr.cli.Close()
			close(leaseWatchChan)
			return ctx.Err()
		case wresp := <-rch:
			batch := make([]LeaseWatchResult, 0)
			for _, etcdEvent := range wresp.Events {
				subnetEvent, err := parseSubnetWatchResponse(ctx, esr.cli, etcdEvent)
				switch {
				case err == nil:
					wr := subnet.LeaseWatchResult{
						Events: []subnet.Event{subnetEvent},
						Cursor: watchCursor{wresp.Header.Revision},
					}
					batch = append(batch, wr)
				case isIndexTooSmall(err):
					log.Warning("Watch of subnet leases failed because etcd index outside history window")
					wr, err := esr.leasesWatchReset(ctx)
					if err != nil {
						log.Errorf("error resetting etcd watch: %s", err)
					}
					batch = append(batch, wr)
				default:
					log.Errorf("couldn't read etcd event: %s", err)
				}
			}
			if len(batch) > 0 {
				leaseWatchChan <- batch
			}
		}

	}
}

func (esr *etcdSubnetRegistry) kv() etcd.KV {
	esr.mux.Lock()
	defer esr.mux.Unlock()
	return esr.kvApi
}

func parseSubnetWatchResponse(ctx context.Context, cli *etcd.Client, ev *etcd.Event) (Event, error) {
	sn, tsn6 := ParseSubnetKey(string(ev.Kv.Key))
	if sn == nil {
		return Event{}, fmt.Errorf("%v %q: not a subnet, skipping", ev.Type, string(ev.Kv.Key))
	}

	var sn6 ip.IP6Net
	if tsn6 != nil {
		sn6 = *tsn6
	}

	switch ev.Type {
	case etcd.EventTypeDelete:
		return Event{
			Type: EventRemoved,
			Lease: Lease{
				EnableIPv4: true,
				Subnet:     *sn,
				EnableIPv6: !sn6.Empty(),
				IPv6Subnet: sn6,
			},
		}, nil

	default:
		attrs := &LeaseAttrs{}
		err := json.Unmarshal(ev.Kv.Value, attrs)
		if err != nil {
			return Event{}, err
		}

		lresp, lerr := cli.TimeToLive(ctx, etcd.LeaseID(ev.Kv.Lease))
		if lerr != nil {
			return Event{}, lerr
		}
		exp := time.Now().Add(time.Duration(lresp.TTL) * time.Second)
		evt := Event{
			Type: EventAdded,
			Lease: Lease{
				EnableIPv4: true,
				Subnet:     *sn,
				EnableIPv6: !sn6.Empty(),
				IPv6Subnet: sn6,
				Attrs:      *attrs,
				Expiration: exp,
			},
		}
		return evt, nil
	}
}

func kvToIPLease(kv *mvccpb.KeyValue, ttl int64) (*Lease, error) {
	sn, tsn6 := ParseSubnetKey(string(kv.Key))
	if sn == nil {
		return nil, fmt.Errorf("failed to parse subnet key %s", kv.Key)
	}

	var sn6 ip.IP6Net
	if tsn6 != nil {
		sn6 = *tsn6
	}

	attrs := &LeaseAttrs{}
	if err := json.Unmarshal([]byte(kv.Value), attrs); err != nil {
		return nil, err
	}

	exp := time.Now().Add(time.Duration(ttl) * time.Second)

	lease := Lease{
		EnableIPv4: true,
		EnableIPv6: !sn6.Empty(),
		Subnet:     *sn,
		IPv6Subnet: sn6,
		Attrs:      *attrs,
		Expiration: exp,
		Asof:       kv.ModRevision,
	}

	return &lease, nil
}

// leasesWatchReset is called when incremental lease watch failed and we need to grab a snapshot
func (esr *etcdSubnetRegistry) leasesWatchReset(ctx context.Context) (subnet.LeaseWatchResult, error) {
	wr := subnet.LeaseWatchResult{}

	leases, index, err := esr.getSubnets(ctx)
	if err != nil {
		return wr, fmt.Errorf("failed to retrieve subnet leases: %v", err)
	}

	wr.Cursor = watchCursor{index}
	wr.Snapshot = leases
	return wr, nil
}
