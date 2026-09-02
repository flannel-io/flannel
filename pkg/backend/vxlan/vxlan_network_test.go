// Copyright 2026 flannel authors
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

//go:build !windows
// +build !windows

package vxlan

import (
	"context"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestWatchLinkUpdatesSignalsDeletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan netlink.LinkUpdate)
	missing := make(chan bool, 1)
	closed := make(chan bool)
	go func() { closed <- watchLinkUpdates(ctx, updates, "flannel.1", missing) }()

	link := &netlink.Vxlan{LinkAttrs: netlink.LinkAttrs{Name: "flannel.1"}}
	other := &netlink.Vxlan{LinkAttrs: netlink.LinkAttrs{Name: "flannel-v6.1"}}
	updates <- netlink.LinkUpdate{Header: unix.NlMsghdr{Type: unix.RTM_NEWLINK}, Link: link}
	updates <- netlink.LinkUpdate{Header: unix.NlMsghdr{Type: unix.RTM_DELLINK}, Link: other}
	select {
	case <-missing:
		t.Fatal("signalled deletion of another link")
	case <-time.After(50 * time.Millisecond):
	}

	updates <- netlink.LinkUpdate{Header: unix.NlMsghdr{Type: unix.RTM_DELLINK}, Link: link}
	select {
	case <-missing:
	case <-time.After(time.Second):
		t.Fatal("deletion of the vxlan device was not signalled")
	}

	cancel()
	if <-closed {
		t.Fatal("watchLinkUpdates reported a closed channel on context cancellation")
	}
}

func TestWatchLinkUpdatesReturnsWhenSubscriptionCloses(t *testing.T) {
	updates := make(chan netlink.LinkUpdate)
	missing := make(chan bool, 1)
	closed := make(chan bool)
	go func() { closed <- watchLinkUpdates(context.Background(), updates, "flannel.1", missing) }()

	close(updates)
	select {
	case c := <-closed:
		if !c {
			t.Fatal("watchLinkUpdates did not report the closed channel")
		}
	case <-time.After(time.Second):
		t.Fatal("watchLinkUpdates did not return after the subscription closed")
	}
}
