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

package subnet

import (
	"fmt"
	golog "log"
	"os"
	"path"
	"sync"
	"time"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
)

type Registry interface {
	getConfig(ctx context.Context, network string) (*etcd.Response, error)
	getSubnets(ctx context.Context, network string) (*etcd.Response, error)
	createSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error)
	updateSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error)
	watchSubnets(ctx context.Context, network string, since uint64) (*etcd.Response, error)
}

type EtcdConfig struct {
	Endpoints []string
	Keyfile   string
	Certfile  string
	CAFile    string
	Prefix    string
}

type etcdSubnetRegistry struct {
	mux     sync.Mutex
	cli     *etcd.Client
	etcdCfg *EtcdConfig
}

func init() {
	etcd.SetLogger(golog.New(os.Stderr, "go-etcd", golog.LstdFlags))
}

func newEtcdClient(c *EtcdConfig) (*etcd.Client, error) {
	if c.Keyfile != "" || c.Certfile != "" || c.CAFile != "" {
		return etcd.NewTLSClient(c.Endpoints, c.Certfile, c.Keyfile, c.CAFile)
	} else {
		return etcd.NewClient(c.Endpoints), nil
	}
}

func newEtcdSubnetRegistry(config *EtcdConfig) (Registry, error) {
	r := &etcdSubnetRegistry{
		etcdCfg: config,
	}

	var err error
	r.cli, err = newEtcdClient(config)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (esr *etcdSubnetRegistry) getConfig(ctx context.Context, network string) (*etcd.Response, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "config")
	resp, err := esr.client().Get(key, false, false)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (esr *etcdSubnetRegistry) getSubnets(ctx context.Context, network string) (*etcd.Response, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets")
	return esr.client().Get(key, false, true)
}

func (esr *etcdSubnetRegistry) createSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", sn)
	resp, err := esr.client().Create(key, data, ttl)
	if err != nil {
		return nil, err
	}

	ensureExpiration(resp, ttl)
	return resp, nil
}

func (esr *etcdSubnetRegistry) updateSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error) {
	key := path.Join(esr.etcdCfg.Prefix, network, "subnets", sn)
	resp, err := esr.client().Set(key, data, ttl)
	if err != nil {
		return nil, err
	}

	ensureExpiration(resp, ttl)
	return resp, nil
}

type watchResp struct {
	resp *etcd.Response
	err  error
}

func (esr *etcdSubnetRegistry) watchSubnets(ctx context.Context, network string, since uint64) (*etcd.Response, error) {
	stop := make(chan bool)
	respCh := make(chan watchResp)

	go func() {
		for {
			key := path.Join(esr.etcdCfg.Prefix, network, "subnets")
			rresp, err := esr.client().RawWatch(key, since, true, nil, stop)

			if err != nil {
				respCh <- watchResp{nil, err}
				return
			}

			if len(rresp.Body) == 0 {
				// etcd timed out, go back but recreate the client as the underlying
				// http transport gets hosed (http://code.google.com/p/go/issues/detail?id=8648)
				esr.resetClient()
				continue
			}

			resp, err := rresp.Unmarshal()
			respCh <- watchResp{resp, err}
		}
	}()

	select {
	case <-ctx.Done():
		close(stop)
		<-respCh // Wait for f to return.
		return nil, ctx.Err()
	case wr := <-respCh:
		return wr.resp, wr.err
	}
}

func (esr *etcdSubnetRegistry) client() *etcd.Client {
	esr.mux.Lock()
	defer esr.mux.Unlock()
	return esr.cli
}

func (esr *etcdSubnetRegistry) resetClient() {
	esr.mux.Lock()
	defer esr.mux.Unlock()

	var err error
	esr.cli.Close()
	esr.cli, err = newEtcdClient(esr.etcdCfg)
	if err != nil {
		panic(fmt.Errorf("resetClient: error recreating etcd client: %v", err))
	}
}

func ensureExpiration(resp *etcd.Response, ttl uint64) {
	if resp.Node.Expiration == nil {
		// should not be but calc it ourselves in this case
		log.Info("Expiration field missing on etcd response, calculating locally")
		exp := time.Now().Add(time.Duration(ttl) * time.Second)
		resp.Node.Expiration = &exp
	}
}
