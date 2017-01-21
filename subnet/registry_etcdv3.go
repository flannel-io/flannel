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
	"fmt"
	"path"
	"regexp"
	"sync"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/pkg/ip"
)

//var (
//subnetRegex *regexp.Regexp = regexp.MustCompile(`(\d+\.\d+.\d+.\d+)-(\d+)`)
//errTryAgain                = errors.New("try again")
//)

type etcdV3NewFunc func(c *EtcdConfig) (*etcd.Client, error)

type etcdV3SubnetRegistry struct {
	cliNewFunc   etcdV3NewFunc
	mux          sync.Mutex
	cli          *etcd.Client
	etcdCfg      *EtcdConfig
	networkRegex *regexp.Regexp
}

func newEtcdV3Client(c *EtcdConfig) (*etcd.Client, error) {
	cli, err := etcd.New(etcd.Config{
		Endpoints:   []string{"localhost:2379", "localhost:22379", "localhost:32379"},
		DialTimeout: 5 * time.Second,
	})

	//tlsInfo := transport.TLSInfo{
	//CertFile: c.Certfile,
	//KeyFile:  c.Keyfile,
	//CAFile:   c.CAFile,
	//}

	//t, err := transport.NewTransport(tlsInfo, time.Second)
	//if err != nil {
	//return nil, err
	//}

	//cli, err := etcd.New(etcd.Config{
	//Endpoints: c.Endpoints,
	//Transport: t,
	//Username:  c.Username,
	//Password:  c.Password,
	//})
	if err != nil {
		return nil, err
	}

	return cli, nil
}

func newEtcdV3SubnetRegistry(config *EtcdConfig, cliNewFunc etcdV3NewFunc) (Registry, error) {
	r := &etcdV3SubnetRegistry{
		etcdCfg:      config,
		networkRegex: regexp.MustCompile(config.Prefix + `/([^/]*)(/|/config)?$`),
	}
	if cliNewFunc != nil {
		r.cliNewFunc = cliNewFunc
	} else {
		r.cliNewFunc = newEtcdV3Client
	}

	var err error
	r.cli, err = r.cliNewFunc(config)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (esr *etcdV3SubnetRegistry) getNetworkConfig(ctx context.Context, network string) (string, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "config")
	//TODO - context, quorum=true

	resp, err := esr.cli.Get(context.TODO(), key)

	if err != nil {
		return "", err
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("Network not found at path: %s", key)
	}
	return string(resp.Kvs[0].Value), nil
}

// getSubnets queries etcd to get a list of currently allocated leases for a given network.
// It returns the leases along with the "as-of" etcd-index that can be used as the starting
// point for etcd watch.
func (esr *etcdV3SubnetRegistry) getSubnets(ctx context.Context, network string) ([]Lease, uint64, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets")
	//TODO - context, quorum=true
	resp, err := esr.cli.Get(context.TODO(), key)

	if err != nil {
		return nil, 0, err
	}

	if len(resp.Kvs) == 0 {
		// key not found: treat it as empty set
		return []Lease{}, uint64(resp.Header.Revision), nil
	}

	leases := []Lease{}
	for _, kvs := range resp.Kvs {
		l, err := nodeToLease(string(kvs.Key), kvs.Value, uint64(kvs.ModRevision))
		if err != nil {
			log.Warningf("Ignoring bad subnet node: %v", err)
			continue
		}

		leases = append(leases, *l)
	}

	return leases, uint64(resp.Header.Revision), nil
}

func (esr *etcdV3SubnetRegistry) getSubnet(ctx context.Context, network string, sn ip.IP4Net) (*Lease, uint64, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	//TODO - context, quorum=true
	resp, err := esr.cli.Get(context.TODO(), key)

	if err != nil {
		return nil, 0, err
	}
	//return string(resp.Kvs[0].Value), nil
	l, err := nodeToLease(string(resp.Kvs[0].Key), resp.Kvs[0].Value, uint64(resp.Kvs[0].ModRevision))
	return l, uint64(resp.Header.Revision), err
}

func (esr *etcdV3SubnetRegistry) createSubnet(ctx context.Context, network string, sn ip.IP4Net, attrs *LeaseAttrs, ttl time.Duration) (time.Time, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	value, err := json.Marshal(attrs)
	if err != nil {
		return time.Time{}, err
	}
	// TODO
	//opts := &etcd.SetOptions{
	//PrevExist: etcd.PrevNoExist,
	//TTL:       ttl,
	//}
	_, err = esr.cli.Put(context.TODO(), key, string(value))

	if err != nil {
		return time.Time{}, err
	}

	exp := time.Time{}
	//if resp.Node.Expiration != nil {
	//exp = *resp.Node.Expiration
	//}

	return exp, nil
}

func (esr *etcdV3SubnetRegistry) updateSubnet(ctx context.Context, network string, sn ip.IP4Net, attrs *LeaseAttrs, ttl time.Duration, asof uint64) (time.Time, error) {
	panic("")
	//key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	return time.Time{}, nil
	//value, err := json.Marshal(attrs)
	//if err != nil {
	//return time.Time{}, err
	//}

	//resp, err := esr.client().Set(ctx, key, string(value), &etcd.SetOptions{
	//PrevIndex: asof,
	//TTL:       ttl,
	//})
	//if err != nil {
	//return time.Time{}, err
	//}

	//exp := time.Time{}
	//if resp.Node.Expiration != nil {
	//exp = *resp.Node.Expiration
	//}

	//return exp, nil
}

func (esr *etcdV3SubnetRegistry) deleteSubnet(ctx context.Context, network string, sn ip.IP4Net) error {
	panic("")
	//key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	//_, err := esr.client().Delete(ctx, key, nil)
	//return err
	return nil
}

func (esr *etcdV3SubnetRegistry) watchSubnets(ctx context.Context, network string, since uint64) (Event, uint64, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets")
	// TODO - not sure about the watchprefix stuff
	since++
	rch := esr.cli.Watch(context.Background(), key, etcd.WithPrefix(), etcd.WithRev(int64(since)))
	for wresp := range rch {
		if wresp.Canceled {
			return Event{}, 0, wresp.Err()
		}
		for _, ev := range wresp.Events {

			fmt.Printf("SUBNETS: %s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			evt, err := parseSubnetWatchResponseV3(ev)
			return evt, uint64(ev.Kv.ModRevision), err
		}
	}
	return Event{}, 0, nil
}

func (esr *etcdV3SubnetRegistry) watchSubnet(ctx context.Context, network string, since uint64, sn ip.IP4Net) (Event, uint64, error) {
	since++
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", MakeSubnetKey(sn))
	rch := esr.cli.Watch(context.Background(), key, etcd.WithRev(int64(since)))
	for wresp := range rch {
		if wresp.Canceled {
			return Event{}, 0, wresp.Err()
		}
		for _, ev := range wresp.Events {

			fmt.Printf("SUBNET: %s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			evt, err := parseSubnetWatchResponseV3(ev)
			return evt, uint64(ev.Kv.ModRevision), err
		}
	}
	return Event{}, 0, nil
}

// getNetworks queries etcd to get a list of network names.  It returns the
// networks along with the 'as-of' etcd-index that can be used as the starting
// point for etcd watch.
func (esr *etcdV3SubnetRegistry) getNetworks(ctx context.Context) ([]string, uint64, error) {
	panic("")
	//resp, err := esr.client().Get(ctx, esr.etcdCfg.Prefix, &etcd.GetOptions{Recursive: true, Quorum: true})

	//networks := []string{}

	//if err == nil {
	//for _, node := range resp.Node.Nodes {
	//// Look for '/config' on the child nodes
	//for _, child := range node.Nodes {
	//netname, isConfig := esr.parseNetworkKey(child.Key)
	//if isConfig {
	//networks = append(networks, netname)
	//}
	//}
	//}

	//return networks, resp.Index, nil
	//}

	//if etcdErr, ok := err.(etcd.Error); ok && etcdErr.Code == etcd.ErrorCodeKeyNotFound {
	//// key not found: treat it as empty set
	//return networks, etcdErr.Index, nil
	//}

	return nil, 0, nil
}

func (esr *etcdV3SubnetRegistry) watchNetworks(ctx context.Context, since uint64) (Event, uint64, error) {
	panic("")
	return Event{}, 0, nil
	//key := esr.etcdCfg.Prefix
	//opts := &etcd.WatcherOptions{
	//AfterIndex: since,
	//Recursive:  true,
	//}
	//e, err := esr.client().Watcher(key, opts).Next(ctx)
	//if err != nil {
	//return Event{}, 0, err
	//}

	//return esr.parseNetworkWatchResponse(e)
}

//func (esr *etcdV3SubnetRegistry) client() etcd.KeysAPI {
//esr.mux.Lock()
//defer esr.mux.Unlock()
//return esr.cli
//}

//func (esr *etcdV3SubnetRegistry) resetClient() {
//esr.mux.Lock()
//defer esr.mux.Unlock()

//var err error
//esr.cli, err = newEtcdClient(esr.etcdCfg)
//if err != nil {
//panic(fmt.Errorf("resetClient: error recreating etcd client: %v", err))
//}
//}

func parseSubnetWatchResponseV3(resp *etcd.Event) (Event, error) {
	sn := ParseSubnetKey(string(resp.Kv.Key))
	if sn == nil {
		return Event{}, fmt.Errorf("%v %q: not a subnet, skipping", resp, resp.Kv.Key)
	}

	switch resp.Type {

	//TODO
	//case "delete", "expire":
	//return Event{
	//EventRemoved,
	//Lease{Subnet: *sn},
	//"",
	//}, nil

	default:
		attrs := &LeaseAttrs{}
		err := json.Unmarshal(resp.Kv.Value, attrs)
		if err != nil {
			return Event{}, err
		}

		exp := time.Time{}
		//if resp.Node.Expiration != nil {
		//exp = *resp.Node.Expiration
		//}

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

//func (esr *etcdV3SubnetRegistry) parseNetworkWatchResponse(resp *etcd.Response) (Event, uint64, error) {
//index := resp.Node.ModifiedIndex
//netname, isConfig := esr.parseNetworkKey(resp.Node.Key)
//if netname == "" {
//return Event{}, index, errTryAgain
//}

//evt := Event{}

//switch resp.Action {
//case "delete":
//evt = Event{
//EventRemoved,
//Lease{},
//netname,
//}

//default:
//if !isConfig {
//// Ignore non .../<netname>/config keys; tell caller to try again
//return Event{}, index, errTryAgain
//}

//_, err := ParseConfig(resp.Node.Value)
//if err != nil {
//return Event{}, index, err
//}

//evt = Event{
//EventAdded,
//Lease{},
//netname,
//}
//}

//return evt, index, nil
//}

//// Returns network name from config key (eg, /coreos.com/network/foobar/config),
//// if the 'config' key isn't present we don't consider the network valid
//func (esr *etcdV3SubnetRegistry) parseNetworkKey(s string) (string, bool) {
//if parts := esr.networkRegex.FindStringSubmatch(s); len(parts) == 3 {
//return parts[1], parts[2] != ""
//}

//return "", false
//}
