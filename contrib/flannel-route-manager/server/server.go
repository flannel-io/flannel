package server

import (
	"encoding/json"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/kelseyhightower/flannel-route-manager/backend"
)

type routeInfo struct {
	PublicIP string
}

type Server struct {
	client       *etcd.Client
	lastIndex    uint64
	mu           sync.Mutex
	prefix       string
	routeManager backend.RouteManager
	stopChan     chan bool
	syncInterval int
	wg           sync.WaitGroup
}

func New(etcdEndpoint, prefix string, syncInterval int, routeManager backend.RouteManager) *Server {
	return &Server{
		client:       etcd.NewClient([]string{etcdEndpoint}),
		prefix:       path.Join(prefix, "subnets"),
		routeManager: routeManager,
		stopChan:     make(chan bool),
		syncInterval: syncInterval,
	}
}

func (s *Server) Start() *Server {
	s.syncAllRoutes()
	go s.monitorSubnets()
	go s.reconciler()
	return s
}

func (s *Server) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *Server) syncAllRoutes() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	routeTable := make(map[string]string)
	resp, err := s.client.Get(s.prefix, false, true)
	if err != nil {
		return err
	}
	s.lastIndex = resp.EtcdIndex
	for _, node := range resp.Node.Nodes {
		subnet := strings.Replace(path.Base(node.Key), "-", "/", -1)
		var ri routeInfo
		err := json.Unmarshal([]byte(node.Value), &ri)
		if err != nil {
			return err
		}
		routeTable[ri.PublicIP] = subnet
	}
	log.Printf("reconciler starting...")
	defer log.Printf("reconciler done")
	syncResp, err := s.routeManager.Sync(routeTable)
	if resp != nil {
		for _, r := range syncResp.Inserted {
			log.Printf("reconciler: inserted %s\n", r)
		}
		for _, r := range syncResp.Deleted {
			log.Printf("reconciler: deleted %s\n", r)
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) syncRoute(resp *etcd.Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subnet := strings.Replace(path.Base(resp.Node.Key), "-", "/", -1)
	switch resp.Action {
	case "create", "set", "update":
		var ri routeInfo
		err := json.Unmarshal([]byte(resp.Node.Value), &ri)
		if err != nil {
			log.Println(err.Error())
			return
		}
		name, err := s.routeManager.Insert(ri.PublicIP, subnet)
		if err != nil {
			log.Println(err.Error())
			return
		}
		log.Printf("monitor: inserted %s\n", name)
	case "delete":
		name, err := s.routeManager.Delete(subnet)
		if err != nil {
			log.Println(err.Error())
			return
		}
		log.Printf("monitor: deleted %s\n", name)
	default:
		log.Printf("unknown etcd action: %s\n", resp.Action)
	}
}

func (s *Server) monitorSubnets() {
	s.wg.Add(1)
	defer s.wg.Done()
	doneChan := make(chan struct{})
	stopWatchChan := make(chan bool)
	respChan := make(chan *etcd.Response)
	go func() {
		defer close(doneChan)
		for {
			resp, err := s.client.Watch(s.prefix, s.lastIndex+1, true, nil, stopWatchChan)
			if err != nil {
				_, ok := <-stopWatchChan
				if !ok {
					log.Println("stopping etcd watch...")
					return
				}
				log.Println(err.Error())
				time.Sleep(10 * time.Second)
				continue
			}
			s.lastIndex = resp.Node.ModifiedIndex
			respChan <- resp
		}
	}()
	for {
		select {
		case resp := <-respChan:
			s.syncRoute(resp)
		case <-s.stopChan:
			close(stopWatchChan)
			<-doneChan
			log.Println("stopping monitorSubnets...")
			return
		}
	}
}

func (s *Server) reconciler() {
	s.wg.Add(1)
	defer s.wg.Done()
	for {
		select {
		case <-s.stopChan:
			log.Println("stopping reconciler...")
			return
		case <-time.After(time.Duration(s.syncInterval) * time.Second):
			s.syncAllRoutes()
		}
	}
}
