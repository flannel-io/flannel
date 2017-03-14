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

package subnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sync"
	"time"

	etcd "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/pkg/ip"
)

var (
	subnetRegex *regexp.Regexp = regexp.MustCompile(`(\d+\.\d+.\d+.\d+)-(\d+)`)
	errTryAgain                = errors.New("try again")
)

type etcdNewFunc func(c *EtcdConfig) (etcd.KeysAPI, error)

type etcdV2SubnetRegistry struct {
	cliNewFunc   etcdNewFunc
	mux          sync.Mutex
	cli          etcd.KeysAPI
	etcdCfg      *EtcdConfig
	networkRegex *regexp.Regexp
}

func newEtcdClient(c *EtcdConfig) (etcd.KeysAPI, error) {
	tlsInfo := transport.TLSInfo{
		CertFile: c.Certfile,
		KeyFile:  c.Keyfile,
		CAFile:   c.CAFile,
	}

	t, err := transport.NewTransport(tlsInfo, time.Second)
	if err != nil {
		return nil, err
	}

	cli, err := etcd.New(etcd.Config{
		Endpoints: c.Endpoints,
		Transport: t,
		Username:  c.Username,
		Password:  c.Password,
	})
	if err != nil {
		return nil, err
	}

	return etcd.NewKeysAPI(cli), nil
}

func newEtcdV2SubnetRegistry(config *EtcdConfig, cliNewFunc etcdNewFunc) (Registry, error) {
	r := &etcdV2SubnetRegistry{
		etcdCfg:      config,
		networkRegex: regexp.MustCompile(config.Prefix + `/([^/]*)(/|/config)?$`),
	}
	if cliNewFunc != nil {
		r.cliNewFunc = cliNewFunc
	} else {
		r.cliNewFunc = newEtcdClient
	}

	var err error
	r.cli, err = r.cliNewFunc(config)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (esr *etcdV2SubnetRegistry) getNetworkConfig(ctx context.Context, network string) (string, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "config")
	resp, err := esr.client().Get(ctx, key, &etcd.GetOptions{Quorum: true})
	if err != nil {
		return "", err
	}
	return resp.Node.Value, nil
}

// getSubnets queries etcd to get a list of currently allocated leases for a given network.
// It returns the leases along with the "as-of" etcd-index that can be used as the starting
// point for etcd watch.
func (esr *etcdV2SubnetRegistry) getSubnets(ctx context.Context, network string) ([]Lease, uint64, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets")
	resp, err := esr.client().Get(ctx, key, &etcd.GetOptions{Recursive: true, Quorum: true})
	if err != nil {
		if etcdErr, ok := err.(etcd.Error); ok && etcdErr.Code == etcd.ErrorCodeKeyNotFound {
			// key not found: treat it as empty set
			return []Lease{}, etcdErr.Index, nil
		}
		return nil, 0, err
	}

	leases := []Lease{}
	for _, node := range resp.Node.Nodes {
		l, err := nodeToLease(node.Key, []byte(node.Value), node.ModifiedIndex)
		if err != nil {
			log.Warningf("Ignoring bad subnet node: %v", err)
			continue
		}

		leases = append(leases, *l)
	}

	return leases, resp.Index, nil
}

func (esr *etcdV2SubnetRegistry) getSubnet(ctx context.Context, network string, sn ip.IP4Net) (*Lease, uint64, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	resp, err := esr.client().Get(ctx, key, &etcd.GetOptions{Quorum: true})
	if err != nil {
		return nil, 0, err
	}

	node := resp.Node
	l, err := nodeToLease(node.Key, []byte(node.Value), node.ModifiedIndex)
	return l, resp.Index, err
}

func (esr *etcdV2SubnetRegistry) createSubnet(ctx context.Context, network string, sn ip.IP4Net, attrs *LeaseAttrs, ttl time.Duration) (time.Time, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	value, err := json.Marshal(attrs)
	if err != nil {
		return time.Time{}, err
	}

	opts := &etcd.SetOptions{
		PrevExist: etcd.PrevNoExist,
		TTL:       ttl,
	}

	resp, err := esr.client().Set(ctx, key, string(value), opts)
	if err != nil {
		return time.Time{}, err
	}

	exp := time.Time{}
	if resp.Node.Expiration != nil {
		exp = *resp.Node.Expiration
	}

	return exp, nil
}

func (esr *etcdV2SubnetRegistry) updateSubnet(ctx context.Context, network string, sn ip.IP4Net, attrs *LeaseAttrs, ttl time.Duration, asof uint64) (time.Time, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	value, err := json.Marshal(attrs)
	if err != nil {
		return time.Time{}, err
	}

	resp, err := esr.client().Set(ctx, key, string(value), &etcd.SetOptions{
		PrevIndex: asof,
		TTL:       ttl,
	})
	if err != nil {
		return time.Time{}, err
	}

	exp := time.Time{}
	if resp.Node.Expiration != nil {
		exp = *resp.Node.Expiration
	}

	return exp, nil
}

func (esr *etcdV2SubnetRegistry) deleteSubnet(ctx context.Context, network string, sn ip.IP4Net) error {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	_, err := esr.client().Delete(ctx, key, nil)
	return err
}

func (esr *etcdV2SubnetRegistry) watchSubnets(ctx context.Context, network string, since uint64) (Event, uint64, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets")
	opts := &etcd.WatcherOptions{
		AfterIndex: since,
		Recursive:  true,
	}
	e, err := esr.client().Watcher(key, opts).Next(ctx)
	if err != nil {
		return Event{}, 0, err
	}

	evt, err := parseSubnetWatchResponse(e)
	return evt, e.Node.ModifiedIndex, err
}

func (esr *etcdV2SubnetRegistry) watchSubnet(ctx context.Context, network string, since uint64, sn ip.IP4Net) (Event, uint64, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	opts := &etcd.WatcherOptions{
		AfterIndex: since,
	}

	e, err := esr.client().Watcher(key, opts).Next(ctx)
	if err != nil {
		return Event{}, 0, err
	}

	evt, err := parseSubnetWatchResponse(e)
	return evt, e.Node.ModifiedIndex, err
}

// getNetworks queries etcd to get a list of network names.  It returns the
// networks along with the 'as-of' etcd-index that can be used as the starting
// point for etcd watch.
func (esr *etcdV2SubnetRegistry) getNetworks(ctx context.Context) ([]string, uint64, error) {
	resp, err := esr.client().Get(ctx, esr.etcdCfg.Prefix, &etcd.GetOptions{Recursive: true, Quorum: true})

	networks := []string{}

	if err == nil {
		for _, node := range resp.Node.Nodes {
			// Look for '/config' on the child nodes
			for _, child := range node.Nodes {
				netname, isConfig := esr.parseNetworkKey(child.Key)
				if isConfig {
					networks = append(networks, netname)
				}
			}
		}

		return networks, resp.Index, nil
	}

	if etcdErr, ok := err.(etcd.Error); ok && etcdErr.Code == etcd.ErrorCodeKeyNotFound {
		// key not found: treat it as empty set
		return networks, etcdErr.Index, nil
	}

	return nil, 0, err
}

func (esr *etcdV2SubnetRegistry) watchNetworks(ctx context.Context, since uint64) (Event, uint64, error) {
	key := esr.etcdCfg.Prefix
	opts := &etcd.WatcherOptions{
		AfterIndex: since,
		Recursive:  true,
	}
	e, err := esr.client().Watcher(key, opts).Next(ctx)
	if err != nil {
		return Event{}, 0, err
	}

	return esr.parseNetworkWatchResponse(e)
}

func (esr *etcdV2SubnetRegistry) client() etcd.KeysAPI {
	esr.mux.Lock()
	defer esr.mux.Unlock()
	return esr.cli
}

func (esr *etcdV2SubnetRegistry) resetClient() {
	esr.mux.Lock()
	defer esr.mux.Unlock()

	var err error
	esr.cli, err = newEtcdClient(esr.etcdCfg)
	if err != nil {
		panic(fmt.Errorf("resetClient: error recreating etcd client: %v", err))
	}
}

func parseSubnetWatchResponse(resp *etcd.Response) (Event, error) {
	sn := ParseSubnetKey(resp.Node.Key)
	if sn == nil {
		return Event{}, fmt.Errorf("%v %q: not a subnet, skipping", resp.Action, resp.Node.Key)
	}

	switch resp.Action {
	case "delete", "expire":
		return Event{
			EventRemoved,
			Lease{Subnet: *sn},
			"",
		}, nil

	default:
		attrs := &LeaseAttrs{}
		err := json.Unmarshal([]byte(resp.Node.Value), attrs)
		if err != nil {
			return Event{}, err
		}

		exp := time.Time{}
		if resp.Node.Expiration != nil {
			exp = *resp.Node.Expiration
		}

		evt := Event{
			EventAdded,
			Lease{
				Subnet:     *sn,
				Attrs:      *attrs,
				Expiration: exp,
			},
			"",
		}
		return evt, nil
	}
}

func (esr *etcdV2SubnetRegistry) parseNetworkWatchResponse(resp *etcd.Response) (Event, uint64, error) {
	index := resp.Node.ModifiedIndex
	netname, isConfig := esr.parseNetworkKey(resp.Node.Key)
	if netname == "" {
		return Event{}, index, errTryAgain
	}

	evt := Event{}

	switch resp.Action {
	case "delete":
		evt = Event{
			EventRemoved,
			Lease{},
			netname,
		}

	default:
		if !isConfig {
			// Ignore non .../<netname>/config keys; tell caller to try again
			return Event{}, index, errTryAgain
		}

		_, err := ParseConfig(resp.Node.Value)
		if err != nil {
			return Event{}, index, err
		}

		evt = Event{
			EventAdded,
			Lease{},
			netname,
		}
	}

	return evt, index, nil
}

// Returns network name from config key (eg, /coreos.com/network/foobar/config),
// if the 'config' key isn't present we don't consider the network valid
func (esr *etcdV2SubnetRegistry) parseNetworkKey(s string) (string, bool) {
	if parts := esr.networkRegex.FindStringSubmatch(s); len(parts) == 3 {
		return parts[1], parts[2] != ""
	}

	return "", false
}

// TODO move - shared code
func nodeToLease(key string, value []byte, modifiedIndex uint64) (*Lease, error) {
	sn := ParseSubnetKey(key)
	if sn == nil {
		return nil, fmt.Errorf("failed to parse subnet key %q", *sn)
	}

	attrs := &LeaseAttrs{}
	if err := json.Unmarshal(value, attrs); err != nil {
		return nil, err
	}

	exp := time.Time{}
	//if node.Expiration != nil {
	//exp = *node.Expiration
	//}

	lease := Lease{
		Subnet:     *sn,
		Attrs:      *attrs,
		Expiration: exp,
		asof:       modifiedIndex,
	}

	return &lease, nil
}
