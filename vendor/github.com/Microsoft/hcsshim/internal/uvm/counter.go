package uvm

import "sync"

var (
	m       sync.Mutex
	counter uint64
)

// ContainerCounter is used for where we layout things for a container in
// a utility VM. For WCOW it'll be C:\c\N\. For LCOW it'll be /run/gcs/c/N/.
func (uvm *UtilityVM) ContainerCounter() uint64 {
	m.Lock()
	defer m.Unlock()
	counter++
	return counter
}
