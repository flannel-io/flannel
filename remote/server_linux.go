// +build linux !windows

// Copyright 2016 flannel authors
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

package remote

import (
	"fmt"
	"net"
	"strconv"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/go-systemd/activation"
)

func fdListener(addr string) (net.Listener, error) {
	fdOffset := 0
	if addr != "" {
		fd, err := strconv.Atoi(addr)
		if err != nil {
			return nil, fmt.Errorf("fd index is not a number")
		}
		fdOffset = fd - 3
	}

	listeners, err := activation.Listeners(false)
	if err != nil {
		return nil, err
	}

	if fdOffset >= len(listeners) {
		return nil, fmt.Errorf("fd %v is out of range (%v)", addr, len(listeners)+3)
	}

	if listeners[fdOffset] == nil {
		return nil, fmt.Errorf("fd %v was not socket activated", addr)
	}

	return listeners[fdOffset], nil
}
