package subnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"time"

	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/rudder/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/rudder/pkg/ip"
)

const (
	registerRetries = 10
	subnetTTL       = 24 * 3600
	renewMargin     = time.Hour
)

// etcd error codes
const (
	etcdKeyNotFound       = 100
	etcdKeyAlreadyExists  = 105
	etcdEventIndexCleared = 401
)

const (
	SubnetAdded = iota
	SubnetRemoved
)

var (
	subnetRegex *regexp.Regexp = regexp.MustCompile(`(\d+\.\d+.\d+.\d+)-(\d+)`)
)

type SubnetLease struct {
	Network ip.IP4Net
	Data    string
}

type SubnetManager struct {
	registry  subnetRegistry
	config    *Config
	myLease   SubnetLease
	leaseExp  time.Time
	lastIndex uint64
	leases    []SubnetLease
	stop      chan bool
}

type EventType int

type Event struct {
	Type  EventType
	Lease SubnetLease
}

type EventBatch []Event

func NewSubnetManager(etcdCli *etcd.Client, prefix string) (*SubnetManager, error) {
	esr := etcdSubnetRegistry{etcdCli, prefix}
	return newSubnetManager(&esr)
}

func (sm *SubnetManager) AcquireLease(tep ip.IP4, data string) (ip.IP4Net, error) {
	for i := 0; i < registerRetries; i++ {
		var err error
		sm.leases, err = sm.getLeases()
		if err != nil {
			return ip.IP4Net{}, err
		}

		// try to reuse a subnet if there's one that match our IP
		for _, l := range sm.leases {
			var ba BaseAttrs
			err = json.Unmarshal([]byte(l.Data), &ba)
			if err != nil {
				log.Error("Error parsing subnet lease JSON: ", err)
			} else {
				if tep == ba.PublicIP {
					resp, err := sm.registry.updateSubnet(l.Network.StringSep(".", "-"), data, subnetTTL)
					if err != nil {
						return ip.IP4Net{}, err
					}

					sm.myLease.Network = l.Network
					sm.leaseExp = *resp.Node.Expiration
					return l.Network, nil
				}
			}
		}

		// no existing match, grab a new one
		sn, err := sm.allocateSubnet()
		if err != nil {
			return ip.IP4Net{}, err
		}

		resp, err := sm.registry.createSubnet(sn.StringSep(".", "-"), data, subnetTTL)
		switch {
		case err == nil:
			sm.myLease.Network = sn
			sm.leaseExp = *resp.Node.Expiration
			return sn, nil

		// if etcd returned Key Already Exists, try again.
		case err.(*etcd.EtcdError).ErrorCode == etcdKeyAlreadyExists:
			continue

		default:
			return ip.IP4Net{}, err
		}
	}

	return ip.IP4Net{}, errors.New("Max retries reached trying to acquire a subnet")
}

func (sm *SubnetManager) UpdateSubnet(data string) error {
	resp, err := sm.registry.updateSubnet(sm.myLease.Network.StringSep(".", "-"), data, subnetTTL)
	sm.leaseExp = *resp.Node.Expiration
	return err
}

func (sm *SubnetManager) Start(receiver chan EventBatch) {
	go sm.watchLeases(receiver)
	go sm.leaseRenewer()
}

func (sm *SubnetManager) Stop() {
	// once for each goroutine
	sm.stop <- true
	sm.stop <- true
}

func (sm *SubnetManager) GetConfig() *Config {
	return sm.config
}

/// Implementation

func parseSubnetKey(s string) (ip.IP4Net, error) {
	if parts := subnetRegex.FindStringSubmatch(s); len(parts) == 3 {
		snIp := net.ParseIP(parts[1]).To4()
		prefixLen, err := strconv.ParseUint(parts[2], 10, 5)
		if snIp != nil && err == nil {
			return ip.IP4Net{IP: ip.FromIP(snIp), PrefixLen: uint(prefixLen)}, nil
		}
	}

	return ip.IP4Net{}, errors.New("Error parsing IP Subnet")
}

type subnetRegistry interface {
	getConfig() (*etcd.Response, error)
	getSubnets() (*etcd.Response, error)
	createSubnet(sn, data string, ttl uint64) (*etcd.Response, error)
	updateSubnet(sn, data string, ttl uint64) (*etcd.Response, error)
	watchSubnets(since uint64, stop chan bool) (*etcd.Response, error)
}

type etcdSubnetRegistry struct {
	cli    *etcd.Client
	prefix string
}

func (esr *etcdSubnetRegistry) getConfig() (*etcd.Response, error) {
	resp, err := esr.cli.Get(esr.prefix+"/config", false, false)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (esr *etcdSubnetRegistry) getSubnets() (*etcd.Response, error) {
	return esr.cli.Get(esr.prefix+"/subnets", false, true)
}

func (esr *etcdSubnetRegistry) createSubnet(sn, data string, ttl uint64) (*etcd.Response, error) {
	return esr.cli.Create(esr.prefix+"/subnets/"+sn, data, ttl)
}

func (esr *etcdSubnetRegistry) updateSubnet(sn, data string, ttl uint64) (*etcd.Response, error) {
	return esr.cli.Set(esr.prefix+"/subnets/"+sn, data, ttl)
}

func (esr *etcdSubnetRegistry) watchSubnets(since uint64, stop chan bool) (*etcd.Response, error) {
	return esr.cli.Watch(esr.prefix+"/subnets", since, true, nil, stop)
}

func newSubnetManager(r subnetRegistry) (*SubnetManager, error) {
	cfgResp, err := r.getConfig()
	if err != nil {
		return nil, err
	}

	cfg, err := ParseConfig(cfgResp.Node.Value)
	if err != nil {
		return nil, err
	}

	return &SubnetManager{
		registry: r,
		config:   cfg,
		stop:     make(chan bool, 2),
	}, nil
}

func (sm *SubnetManager) getLeases() ([]SubnetLease, error) {
	resp, err := sm.registry.getSubnets()

	var leases []SubnetLease
	switch {
	case err == nil:
		for _, node := range resp.Node.Nodes {
			sn, err := parseSubnetKey(node.Key)
			if err == nil {
				lease := SubnetLease{sn, node.Value}
				leases = append(leases, lease)
			}
		}
		sm.lastIndex = resp.EtcdIndex

	case err.(*etcd.EtcdError).ErrorCode == etcdKeyNotFound:
		// key not found: treat it as empty set
		sm.lastIndex = err.(*etcd.EtcdError).Index

	default:
		return nil, err
	}

	return leases, nil
}

func deleteLease(l []SubnetLease, i int) []SubnetLease {
	l[i], l = l[len(l)-1], l[:len(l)-1]
	return l
}

func (sm *SubnetManager) applyLeases(newLeases []SubnetLease) EventBatch {
	var batch EventBatch

	for _, l := range newLeases {
		// skip self
		if l.Network.Equal(sm.myLease.Network) {
			continue
		}

		found := false
		for i, c := range sm.leases {
			if c.Network.Equal(l.Network) {
				sm.leases = deleteLease(sm.leases, i)
				found = true
				break
			}
		}

		if !found {
			// new subnet
			batch = append(batch, Event{SubnetAdded, l})
		}
	}

	// everything left in sm.leases has been deleted
	for _, c := range sm.leases {
		batch = append(batch, Event{SubnetRemoved, c})
	}

	sm.leases = newLeases

	return batch
}

func (sm *SubnetManager) applySubnetChange(action string, ipn ip.IP4Net, data string) Event {
	switch action {
	case "delete", "expire":
		for i, l := range sm.leases {
			if l.Network.Equal(ipn) {
				deleteLease(sm.leases, i)
				return Event{SubnetRemoved, l}
			}
		}

		log.Errorf("Removed subnet (%s) was not found", ipn)
		return Event{
			SubnetRemoved,
			SubnetLease{ipn, ""},
		}

	default:
		for i, l := range sm.leases {
			if l.Network.Equal(ipn) {
				sm.leases[i] = SubnetLease{ipn, data}
				return Event{SubnetAdded, sm.leases[i]}
			}
		}

		sm.leases = append(sm.leases, SubnetLease{ipn, data})
		return Event{SubnetAdded, sm.leases[len(sm.leases)-1]}
	}
}

type BaseAttrs struct {
	PublicIP ip.IP4
}

func (sm *SubnetManager) allocateSubnet() (ip.IP4Net, error) {
	log.Infof("Picking subnet in range %s ... %s", sm.config.SubnetMin, sm.config.SubnetMax)

	var bag []ip.IP4
	sn := ip.IP4Net{IP: sm.config.SubnetMin, PrefixLen: sm.config.SubnetLen}

OuterLoop:
	for ; sn.IP <= sm.config.SubnetMax && len(bag) < 100; sn = sn.Next() {
		for _, l := range sm.leases {
			if sn.Overlaps(l.Network) {
				continue OuterLoop
			}
		}
		bag = append(bag, sn.IP)
	}

	if len(bag) == 0 {
		return ip.IP4Net{}, errors.New("out of subnets")
	} else {
		i := randInt(0, len(bag))
		return ip.IP4Net{IP: bag[i], PrefixLen: sm.config.SubnetLen}, nil
	}
}

func (sm *SubnetManager) watchLeases(receiver chan EventBatch) {
	// "catch up" by replaying all the leases we discovered during
	// AcquireLease
	var batch EventBatch
	for _, l := range sm.leases {
		if !sm.myLease.Network.Equal(l.Network) {
			batch = append(batch, Event{SubnetAdded, l})
		}
	}
	if len(batch) > 0 {
		receiver <- batch
	}

	for {
		resp, err := sm.registry.watchSubnets(sm.lastIndex+1, sm.stop)

		// watchSubnets exited by stop chan being signaled
		if err == nil && resp == nil {
			return
		}

		var batch *EventBatch
		if err == nil {
			batch, err = sm.parseSubnetWatchResponse(resp)
		} else {
			batch, err = sm.parseSubnetWatchError(err)
		}

		if err != nil {
			log.Errorf("%v", err)
			time.Sleep(time.Second)
			continue
		}

		if batch != nil {
			receiver <- *batch
		}
	}
}

func (sm *SubnetManager) parseSubnetWatchResponse(resp *etcd.Response) (batch *EventBatch, err error) {
	sm.lastIndex = resp.EtcdIndex

	sn, err := parseSubnetKey(resp.Node.Key)
	if err != nil {
		err = fmt.Errorf("Error parsing subnet IP: %s", resp.Node.Key)
		return
	}

	// Don't process our own changes
	if !sm.myLease.Network.Equal(sn) {
		evt := sm.applySubnetChange(resp.Action, sn, resp.Node.Value)
		batch = &EventBatch{evt}
	}

	return
}

func (sm *SubnetManager) parseSubnetWatchError(err error) (batch *EventBatch, out error) {
	etcdErr, ok := err.(*etcd.EtcdError)
	if ok && etcdErr.ErrorCode == etcdEventIndexCleared {
		// etcd maintains a history window for events and it's possible to fall behind.
		// to recover, get the current state and then "diff" against our cache to generate
		// events for the caller
		log.Warning("Watch of subnet leases failed because etcd index outside history window")

		leases, err := sm.getLeases()
		if err == nil {
			lb := sm.applyLeases(leases)
			batch = &lb
		} else {
			out = fmt.Errorf("Failed to retrieve subnet leases: %v", err)
		}
	} else {
		out = fmt.Errorf("Watch of subnet leases failed: %v", err)
	}

	return
}

func (sm *SubnetManager) leaseRenewer() {
	dur := sm.leaseExp.Sub(time.Now()) - renewMargin

	for {
		select {
		case <-time.After(dur):
			resp, err := sm.registry.updateSubnet(sm.myLease.Network.StringSep(".", "-"), sm.myLease.Data, subnetTTL)
			if err != nil {
				log.Error("Error renewing lease (trying again in 1 min): ", err)
				dur = time.Minute
				continue
			}

			sm.leaseExp = *(resp.Node.Expiration)
			log.Info("Lease renewed, new expiration: ", sm.leaseExp)
			dur = sm.leaseExp.Sub(time.Now()) - renewMargin

		case <-sm.stop:
			return
		}
	}
}
