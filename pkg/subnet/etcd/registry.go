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
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/flannel-io/flannel/pkg/subnet"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/tlsutil"
	etcd "go.etcd.io/etcd/client/v3"
	log "k8s.io/klog/v2"
)

var (
	errTryAgain            = errors.New("try again")
	errConfigNotFound      = errors.New("flannel config not found in etcd store. Did you create your config using etcdv3 API?")
	errNoWatchChannel      = errors.New("no watch channel")
	errMalformedWatchEvent = errors.New("malformed subnet watch event")
	errSubnetAlreadyexists = errors.New("subnet already exists")
)

type Registry interface {
	getNetworkConfig(ctx context.Context) (string, error)
	getSubnets(ctx context.Context) ([]lease.Lease, int64, error)
	getSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) (*lease.Lease, int64, error)
	createSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *lease.LeaseAttrs, ttl time.Duration) (time.Time, error)
	updateSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *lease.LeaseAttrs, ttl time.Duration, asof int64) (time.Time, error)
	deleteSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) error
	watchSubnets(ctx context.Context, since int64, leaseWatchChan chan []lease.LeaseWatchResult) error
	watchSubnet(ctx context.Context, since int64, sn ip.IP4Net, sn6 ip.IP6Net, leaseWatchChan chan []lease.LeaseWatchResult) error
	leasesWatchReset(ctx context.Context) (lease.LeaseWatchResult, error)
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
		err := cli.Close()
		if err != nil {
			log.Errorf("Failed to close etcd client: %v", err)
		}
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
func (esr *etcdSubnetRegistry) getSubnets(ctx context.Context) ([]lease.Lease, int64, error) {
	key := path.Join(esr.etcdCfg.Prefix, "subnets")
	resp, err := esr.kv().Get(ctx, key, etcd.WithPrefix())
	if err != nil {
		if err == rpctypes.ErrGRPCKeyNotFound {
			// key not found: treat it as empty set
			return []lease.Lease{}, 0, nil
		}
		return nil, 0, err
	}

	leases := []lease.Lease{}
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

func (esr *etcdSubnetRegistry) getSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net) (*lease.Lease, int64, error) {
	key := path.Join(esr.etcdCfg.Prefix, "subnets", subnet.MakeSubnetKey(sn, sn6))
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

func (esr *etcdSubnetRegistry) createSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *lease.LeaseAttrs, ttl time.Duration) (time.Time, error) {
	key := path.Join(esr.etcdCfg.Prefix, "subnets", subnet.MakeSubnetKey(sn, sn6))
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

func (esr *etcdSubnetRegistry) updateSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, attrs *lease.LeaseAttrs, ttl time.Duration, asof int64) (time.Time, error) {
	key := path.Join(esr.etcdCfg.Prefix, "subnets", subnet.MakeSubnetKey(sn, sn6))
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
	key := path.Join(esr.etcdCfg.Prefix, "subnets", subnet.MakeSubnetKey(sn, sn6))
	_, err := esr.kv().Delete(ctx, key)
	return err
}

// subnetWatch holds the parts that differ between an all-subnets watch and a
// single-subnet watch. runWatch owns the reconnect loop, backoff, compaction
// recovery and revision bookkeeping.
type subnetWatch struct {
	key    string
	since  int64
	ch     chan []lease.LeaseWatchResult
	resync func(ctx context.Context, ch chan []lease.LeaseWatchResult) (int64, error)
}

// runWatch drives an etcd subnet watch, reconnecting with backoff and recovering
// from compaction, until the context is cancelled.
func (esr *etcdSubnetRegistry) runWatch(ctx context.Context, w subnetWatch) error {
	const (
		initialBackoff = 100 * time.Millisecond
		maxBackoff     = 5 * time.Second
	)

	exponentialBackoff := initialBackoff

	// Keep teardown in one place so every cancellation path closes the client
	// and result channel consistently.
	shutdown := func() error {
		if err := esr.cli.Close(); err != nil {
			log.Errorf("Failed to close etcd client: %v", err)
		}
		close(w.ch)
		return ctx.Err()
	}
	waitForRetry := func() bool {
		timer := time.NewTimer(exponentialBackoff)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			exponentialBackoff = min(exponentialBackoff*2, maxBackoff)
			return true
		}
	}

	for {
		wctx, cancel := context.WithCancel(ctx)
		log.V(4).Infof("registry: watching %s starting from rev %d", w.key, w.since)
		rch := esr.cli.Watch(etcd.WithRequireLeader(wctx), w.key, etcd.WithPrefix(), etcd.WithRev(w.since))
		if rch == nil {
			log.Errorf("Failed to establish etcd watch channel")
			cancel()
			if !waitForRetry() {
				return shutdown()
			}
			continue
		}

	innerLoop:
		for {
			select {
			case <-ctx.Done():
				cancel()
				return shutdown()
			case wresp, ok := <-rch:
				err := wresp.Err()
				if !ok || err != nil {
					cancel()
					// If the watch fell behind etcd's compaction horizon, reconnecting
					// at the same revision fails identically forever (mvcc: required
					// revision has been compacted). Re-read current state at a fresh
					// revision and resume from there instead of hot-looping.
					if isCompacted(wresp) {
						log.Warningf("etcd watch for %s fell behind compaction horizon (compact rev %d), resyncing and resuming", w.key, wresp.CompactRevision)
						for {
							next, rerr := w.resync(ctx, w.ch)
							if rerr == nil {
								w.since = next
								exponentialBackoff = initialBackoff
								break innerLoop
							}
							if ctx.Err() != nil {
								return shutdown()
							}
							log.Errorf("failed to resync %s after compaction: %v", w.key, rerr)
							if !waitForRetry() {
								return shutdown()
							}
						}
					}
					if err != nil {
						log.Warningf("etcd watch channel for %s closed with error %v, reconnecting from rev %d...", w.key, err, w.since)
					} else {
						log.Warningf("etcd watch channel for %s closed, reconnecting from rev %d...", w.key, w.since)
					}
					if !waitForRetry() {
						return shutdown()
					}
					break innerLoop
				}
				results, err := esr.watchResults(ctx, wresp)
				if err != nil {
					cancel()
					log.Warningf("couldn't read etcd event for %s: %v; reconnecting from rev %d...", w.key, err, w.since)
					if !waitForRetry() {
						return shutdown()
					}
					break innerLoop
				}
				if len(results) > 0 {
					if err := sendWatchResults(ctx, w.ch, results); err != nil {
						cancel()
						return shutdown()
					}
				}
				exponentialBackoff = initialBackoff
				// Advance only after every event has been handled. An RPC failure while
				// resolving a lease's TTL must be retried from the same revision.
				if wresp.Header.Revision != 0 {
					w.since = wresp.Header.Revision + 1
				}
			}
		}
	}
}

func sendWatchResults(ctx context.Context, ch chan<- []lease.LeaseWatchResult, results []lease.LeaseWatchResult) error {
	select {
	case ch <- results:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (esr *etcdSubnetRegistry) watchSubnets(ctx context.Context, since int64, leaseWatchChan chan []lease.LeaseWatchResult) error {
	return esr.runWatch(ctx, subnetWatch{
		key:    path.Join(esr.etcdCfg.Prefix, "subnets"),
		since:  since,
		ch:     leaseWatchChan,
		resync: esr.resyncWatch,
	})
}

func (esr *etcdSubnetRegistry) watchResults(ctx context.Context, wresp etcd.WatchResponse) ([]lease.LeaseWatchResult, error) {
	results := make([]lease.LeaseWatchResult, 0)
	for _, etcdEvent := range wresp.Events {
		subnetEvent, err := parseSubnetWatchResponse(ctx, esr.cli, etcdEvent)
		switch {
		case errors.Is(err, errMalformedWatchEvent):
			log.Warningf("skipping malformed etcd event: %v", err)
			continue
		case err != nil:
			return nil, err
		}
		results = append(results, lease.LeaseWatchResult{
			Events: []lease.Event{subnetEvent},
			Cursor: watchCursor{wresp.Header.Revision},
		})
	}
	return results, nil
}

func (esr *etcdSubnetRegistry) watchSubnet(ctx context.Context, since int64, sn ip.IP4Net, sn6 ip.IP6Net, leaseWatchChan chan []lease.LeaseWatchResult) error {
	return esr.runWatch(ctx, subnetWatch{
		key:   path.Join(esr.etcdCfg.Prefix, "subnets", subnet.MakeSubnetKey(sn, sn6)),
		since: since,
		ch:    leaseWatchChan,
		resync: func(ctx context.Context, ch chan []lease.LeaseWatchResult) (int64, error) {
			return esr.resyncWatchSubnet(ctx, sn, sn6, ch)
		},
	})
}

func (esr *etcdSubnetRegistry) kv() etcd.KV {
	esr.mux.Lock()
	defer esr.mux.Unlock()
	return esr.kvApi
}

func parseSubnetWatchResponse(ctx context.Context, cli *etcd.Client, ev *etcd.Event) (lease.Event, error) {
	sn, tsn6 := subnet.ParseSubnetKey(string(ev.Kv.Key))
	if sn == nil {
		return lease.Event{}, fmt.Errorf("%w: %v %q is not a subnet", errMalformedWatchEvent, ev.Type, string(ev.Kv.Key))
	}

	var sn6 ip.IP6Net
	if tsn6 != nil {
		sn6 = *tsn6
	}

	switch ev.Type {
	case etcd.EventTypeDelete:
		return lease.Event{
			Type: lease.EventRemoved,
			Lease: lease.Lease{
				EnableIPv4: true,
				Subnet:     *sn,
				EnableIPv6: !sn6.Empty(),
				IPv6Subnet: sn6,
			},
		}, nil

	default:
		attrs := &lease.LeaseAttrs{}
		err := json.Unmarshal(ev.Kv.Value, attrs)
		if err != nil {
			return lease.Event{}, fmt.Errorf("%w: %v", errMalformedWatchEvent, err)
		}

		lresp, lerr := cli.TimeToLive(ctx, etcd.LeaseID(ev.Kv.Lease))
		if lerr != nil {
			return lease.Event{}, lerr
		}
		exp := time.Now().Add(time.Duration(lresp.TTL) * time.Second)
		evt := lease.Event{
			Type: lease.EventAdded,
			Lease: lease.Lease{
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

func kvToIPLease(kv *mvccpb.KeyValue, ttl int64) (*lease.Lease, error) {
	sn, tsn6 := subnet.ParseSubnetKey(string(kv.Key))
	if sn == nil {
		return nil, fmt.Errorf("failed to parse subnet key %s", kv.Key)
	}

	var sn6 ip.IP6Net
	if tsn6 != nil {
		sn6 = *tsn6
	}

	attrs := &lease.LeaseAttrs{}
	if err := json.Unmarshal([]byte(kv.Value), attrs); err != nil {
		return nil, err
	}

	exp := time.Now().Add(time.Duration(ttl) * time.Second)

	lease := lease.Lease{
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

// isCompacted reports whether a watch response failed because its start revision
// has been compacted away by etcd. Reconnecting at the same revision would fail
// identically forever, so the caller must re-list at a fresh revision instead.
func isCompacted(wresp etcd.WatchResponse) bool {
	return wresp.CompactRevision != 0 || errors.Is(wresp.Err(), rpctypes.ErrCompacted)
}

// resyncWatch re-lists all subnet leases, emits the fresh snapshot on ch, and
// returns the revision the caller should resume watching from. Used to recover a
// watch that has fallen behind etcd's compaction horizon.
func (esr *etcdSubnetRegistry) resyncWatch(ctx context.Context, ch chan []lease.LeaseWatchResult) (int64, error) {
	wr, err := esr.leasesWatchReset(ctx)
	if err != nil {
		return 0, err
	}
	if err := sendWatchResults(ctx, ch, []lease.LeaseWatchResult{wr}); err != nil {
		return 0, err
	}
	return getNextIndex(wr.Cursor)
}

// resyncWatchSubnet re-reads a single subnet lease, emits the result on ch and
// returns the revision to resume from. Single-subnet counterpart to resyncWatch.
//
// A lease deleted while the watch was compacted can't be recovered from the
// watch stream, so synthesize the EventRemoved from its absence and leave the
// revoke policy to the caller.
func (esr *etcdSubnetRegistry) resyncWatchSubnet(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, ch chan []lease.LeaseWatchResult) (int64, error) {
	key := path.Join(esr.etcdCfg.Prefix, "subnets", subnet.MakeSubnetKey(sn, sn6))
	resp, err := esr.kv().Get(ctx, key)
	if err != nil {
		return 0, err
	}

	var wr lease.LeaseWatchResult
	if len(resp.Kvs) == 0 {
		wr = lease.LeaseWatchResult{
			Events: []lease.Event{{
				Type: lease.EventRemoved,
				Lease: lease.Lease{
					EnableIPv4: true,
					Subnet:     sn,
					EnableIPv6: !sn6.Empty(),
					IPv6Subnet: sn6,
				},
			}},
			Cursor: watchCursor{resp.Header.Revision},
		}
	} else {
		ttlresp, err := esr.cli.TimeToLive(ctx, etcd.LeaseID(resp.Kvs[0].Lease))
		if err != nil {
			return 0, err
		}
		l, err := kvToIPLease(resp.Kvs[0], ttlresp.TTL)
		if err != nil {
			return 0, err
		}
		wr = lease.LeaseWatchResult{
			Snapshot: []lease.Lease{*l},
			Cursor:   watchCursor{resp.Header.Revision},
		}
	}

	if err := sendWatchResults(ctx, ch, []lease.LeaseWatchResult{wr}); err != nil {
		return 0, err
	}
	return getNextIndex(wr.Cursor)
}

// leasesWatchReset is called when incremental lease watch failed and we need to grab a snapshot
func (esr *etcdSubnetRegistry) leasesWatchReset(ctx context.Context) (lease.LeaseWatchResult, error) {
	wr := lease.LeaseWatchResult{}

	leases, index, err := esr.getSubnets(ctx)
	if err != nil {
		return wr, fmt.Errorf("failed to retrieve subnet leases: %v", err)
	}

	wr.Cursor = watchCursor{index}
	wr.Snapshot = leases
	return wr, nil
}
