package backend

import (
	"fmt"
	"strings"
	"sync"

	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/flannel/subnet"
)

var backendCtors map[string]BackendCtor = make(map[string]BackendCtor)

type Manager interface {
	GetBackend(backendType string) (Backend, error)
	Wait()
}

type manager struct {
	ctx      context.Context
	sm       subnet.Manager
	extIface *ExternalInterface
	mux      sync.Mutex
	active   map[string]Backend
	wg       sync.WaitGroup
}

func NewManager(ctx context.Context, sm subnet.Manager, extIface *ExternalInterface) Manager {
	return &manager{
		ctx:      ctx,
		sm:       sm,
		extIface: extIface,
	}
}

func (bm *manager) GetBackend(backendType string) (Backend, error) {
	bm.mux.Lock()
	defer bm.mux.Unlock()

	betype := strings.ToLower(backendType)
	// see if one is already running
	if be, ok := bm.active[betype]; ok {
		return be, nil
	}

	// first request, need to create and run it
	befunc, ok := backendCtors[betype]
	if !ok {
		return nil, fmt.Errorf("unknown backend type: %v", betype)
	}

	be, err := befunc(bm.sm, bm.extIface)
	if err != nil {
		return nil, err
	}

	bm.wg.Add(1)
	go func() {
		be.Run(bm.ctx)

		// TODO(eyakubovich): this obviosly introduces a race.
		// GetBackend() could get called while we are here.
		// Currently though, all backends' Run exit only
		// on shutdown

		bm.mux.Lock()
		delete(bm.active, betype)
		bm.mux.Unlock()

		bm.wg.Done()
	}()

	return be, nil
}

func (bm *manager) Wait() {
	bm.wg.Wait()
}

func Register(name string, ctor BackendCtor) {
	log.Infof("Register: %v", name)
	backendCtors[name] = ctor
}
