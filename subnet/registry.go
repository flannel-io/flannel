package subnet

import (
	"sync"
	"time"
	"path"

	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/rudder/Godeps/_workspace/src/github.com/golang/glog"
)

type subnetRegistry interface {
	getConfig() (*etcd.Response, error)
	getSubnets() (*etcd.Response, error)
	createSubnet(sn, data string, ttl uint64) (*etcd.Response, error)
	updateSubnet(sn, data string, ttl uint64) (*etcd.Response, error)
	watchSubnets(since uint64, stop chan bool) (*etcd.Response, error)
}

type etcdSubnetRegistry struct {
	mux      sync.Mutex
	cli      *etcd.Client
	endpoint string
	prefix   string
}

func newEtcdSubnetRegistry(endpoint, prefix string) subnetRegistry {
	return &etcdSubnetRegistry{
		cli:      etcd.NewClient([]string{endpoint}),
		endpoint: endpoint,
		prefix:   prefix,
	}
}

func (esr *etcdSubnetRegistry) getConfig() (*etcd.Response, error) {
	key := path.Join(esr.prefix, "config")
	resp, err := esr.client().Get(key, false, false)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (esr *etcdSubnetRegistry) getSubnets() (*etcd.Response, error) {
	key := path.Join(esr.prefix, "subnets")
	return esr.client().Get(key, false, true)
}

func (esr *etcdSubnetRegistry) createSubnet(sn, data string, ttl uint64) (*etcd.Response, error) {
	key := path.Join(esr.prefix, "subnets", sn)
	resp, err := esr.client().Create(key, data, ttl)
	if err != nil {
		return nil, err
	}

	ensureExpiration(resp, ttl)
	return resp, nil
}

func (esr *etcdSubnetRegistry) updateSubnet(sn, data string, ttl uint64) (*etcd.Response, error) {
	key := path.Join(esr.prefix, "subnets", sn)
	resp, err := esr.client().Set(key, data, ttl)
	if err != nil {
		return nil, err
	}

	ensureExpiration(resp, ttl)
	return resp, nil
}

func (esr *etcdSubnetRegistry) watchSubnets(since uint64, stop chan bool) (*etcd.Response, error) {
	for {
		key := path.Join(esr.prefix, "subnets")
		resp, err := esr.client().RawWatch(key, since, true, nil, stop)

		if err != nil {
			if err == etcd.ErrWatchStoppedByUser {
				return nil, nil
			} else {
				return nil, err
			}
		}

		if len(resp.Body) == 0 {
			// etcd timed out, go back but recreate the client as the underlying
			// http transport gets hosed (http://code.google.com/p/go/issues/detail?id=8648)
			esr.resetClient()
			continue
		}

		return resp.Unmarshal()
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
	esr.cli = etcd.NewClient([]string{esr.endpoint})
}

func ensureExpiration(resp *etcd.Response, ttl uint64) {
	if resp.Node.Expiration == nil {
		// should not be but calc it ourselves in this case
		log.Info("Expiration field missing on etcd response, calculating locally")
		exp := time.Now().Add(time.Duration(ttl) * time.Second)
		resp.Node.Expiration = &exp
	}
}
