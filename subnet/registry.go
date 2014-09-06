package subnet

import (
	"sync"

	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
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
	resp, err := esr.client().Get(esr.prefix+"/config", false, false)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (esr *etcdSubnetRegistry) getSubnets() (*etcd.Response, error) {
	return esr.client().Get(esr.prefix+"/subnets", false, true)
}

func (esr *etcdSubnetRegistry) createSubnet(sn, data string, ttl uint64) (*etcd.Response, error) {
	return esr.client().Create(esr.prefix+"/subnets/"+sn, data, ttl)
}

func (esr *etcdSubnetRegistry) updateSubnet(sn, data string, ttl uint64) (*etcd.Response, error) {
	return esr.client().Set(esr.prefix+"/subnets/"+sn, data, ttl)
}

func (esr *etcdSubnetRegistry) watchSubnets(since uint64, stop chan bool) (*etcd.Response, error) {
	for {
		resp, err := esr.client().RawWatch(esr.prefix+"/subnets", since, true, nil, stop)

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
