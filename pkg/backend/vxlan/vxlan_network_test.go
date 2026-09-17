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
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const testTimeout = 2 * time.Second

func testNetwork(name string) *network {
	return &network{
		dev: &vxlanDevice{
			link: &netlink.Vxlan{LinkAttrs: netlink.LinkAttrs{Name: name}},
		},
	}
}

// runWatcher runs watchVXLANDeviceWithSubscriber in a goroutine and returns a
// channel closed when it returns, so tests don't leak the goroutine.
func runWatcher(nw *network, ctx context.Context, vxlanMissingChan chan bool, subscribe linkSubscribeFunc, retryDelay time.Duration) <-chan struct{} {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		nw.watchVXLANDeviceWithSubscriber(ctx, vxlanMissingChan, subscribe, retryDelay)
	}()
	return stopped
}

func requireClosedSoon(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(testTimeout):
		t.Fatalf("timed out waiting for: %s", msg)
	}
}

// TestWatchVXLANDevice_ClosedChannelWaitsBeforeResubscribe verifies that when
// an established updates channel closes (simulating a netlink Receive()
// error such as ENOBUFS), the watcher does not panic or resubscribe
// immediately, and releases the dead attempt's done channel.
func TestWatchVXLANDevice_ClosedChannelWaitsBeforeResubscribe(t *testing.T) {
	nw := testNetwork("flannel.1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const retryDelay = 1 * time.Second

	var calls int32
	firstSubscribe := make(chan struct{})
	var firstDone <-chan struct{}
	subscribe := func(ch chan<- netlink.LinkUpdate, done <-chan struct{}) error {
		if atomic.AddInt32(&calls, 1) == 1 {
			firstDone = done
			close(firstSubscribe)
		}
		// "Succeeds" but its channel dies immediately, as it would under a
		// persistent Receive() failure.
		close(ch)
		return nil
	}

	vxlanMissingChan := make(chan bool, 1)
	stopped := runWatcher(nw, ctx, vxlanMissingChan, subscribe, retryDelay)

	select {
	case <-firstSubscribe:
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for the initial subscribe call")
	}

	// Wait for the dead attempt's done channel to close: proof the watcher
	// has consumed the closed updates channel, run the !ok recovery branch,
	// and entered linkResubscribe - not just that subscribe was called.
	select {
	case <-firstDone:
	case <-time.After(testTimeout):
		t.Fatal("expected the dead subscription's done channel to be closed")
	}

	// The one timing-based assertion: confirm retryDelay is honored rather
	// than resubscribing immediately, anchored after the deterministic wait
	// above.
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 subscribe call while waiting out retryDelay, got %d", got)
	}

	cancel()
	requireClosedSoon(t, stopped, "watcher to exit after context cancellation")

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected still exactly 1 subscribe call after cancellation, got %d", got)
	}

	select {
	case _, ok := <-vxlanMissingChan:
		if ok {
			t.Fatal("vxlanMissingChan: expected it to be closed with no value")
		}
	default:
		t.Fatal("vxlanMissingChan: expected it to be closed after watcher exit")
	}
}

// TestWatchVXLANDevice_ResubscribeFailureThenSuccess verifies that a failed
// resubscribe attempt is retried, not fatal, until it succeeds.
func TestWatchVXLANDevice_ResubscribeFailureThenSuccess(t *testing.T) {
	nw := testNetwork("flannel.1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int32
	subscribe := func(ch chan<- netlink.LinkUpdate, done <-chan struct{}) error {
		switch atomic.AddInt32(&calls, 1) {
		case 1:
			close(ch) // initial subscribe succeeds, then dies immediately
			return nil
		case 2:
			return errors.New("temporary netlink failure") // first resubscribe fails
		default:
			return nil // second resubscribe succeeds and stays open
		}
	}

	vxlanMissingChan := make(chan bool, 1)
	stopped := runWatcher(nw, ctx, vxlanMissingChan, subscribe, 5*time.Millisecond)

	deadline := time.After(testTimeout)
	for atomic.LoadInt32(&calls) < 3 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for 3 subscribe calls, got %d", atomic.LoadInt32(&calls))
		case <-time.After(time.Millisecond):
		}
	}

	cancel()
	requireClosedSoon(t, stopped, "watcher to exit after context cancellation")
}

// TestWaitForRetry_ContextCancellation verifies waitForRetry returns false
// promptly on context cancellation instead of waiting out the full delay.
func TestWaitForRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	result := make(chan bool, 1)

	go func() {
		close(started)
		result <- waitForRetry(ctx, 30*time.Second)
	}()

	<-started
	start := time.Now()
	cancel()

	select {
	case ok := <-result:
		if ok {
			t.Fatal("expected waitForRetry to return false on context cancellation")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("waitForRetry did not return promptly on context cancellation")
	}
	if elapsed := time.Since(start); elapsed >= 30*time.Second {
		t.Fatalf("waitForRetry waited out the full delay instead of exiting promptly: %s", elapsed)
	}
}

// TestWatchVXLANDevice_ContextCancellationDuringRetry_ExitsPromptly verifies
// the watcher exits promptly when canceled while it's waiting to resubscribe,
// exercising the wiring from a dead subscription through to watcher exit.
func TestWatchVXLANDevice_ContextCancellationDuringRetry_ExitsPromptly(t *testing.T) {
	nw := testNetwork("flannel.1")
	ctx, cancel := context.WithCancel(context.Background())

	var calls int32
	firstSubscribe := make(chan struct{})
	var firstDone <-chan struct{}
	subscribe := func(ch chan<- netlink.LinkUpdate, done <-chan struct{}) error {
		if atomic.AddInt32(&calls, 1) == 1 {
			firstDone = done
			close(firstSubscribe)
		}
		close(ch)
		return nil
	}

	const longRetryDelay = 30 * time.Second
	vxlanMissingChan := make(chan bool, 1)
	stopped := runWatcher(nw, ctx, vxlanMissingChan, subscribe, longRetryDelay)

	select {
	case <-firstSubscribe:
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for the initial subscribe call")
	}

	// Wait for the dead attempt's done channel to close: confirms the
	// watcher has processed the dead subscription and entered the
	// resubscription path, rather than racing cancellation against that.
	select {
	case <-firstDone:
	case <-time.After(testTimeout):
		t.Fatal("expected the dead subscription's done channel to be closed")
	}

	start := time.Now()
	cancel()

	select {
	case <-stopped:
	case <-time.After(1 * time.Second):
		t.Fatal("watcher did not exit promptly on context cancellation during retry wait")
	}

	if elapsed := time.Since(start); elapsed >= longRetryDelay {
		t.Fatalf("watcher waited out the full retry delay instead of exiting promptly: %s", elapsed)
	}
}

// TestWatchVXLANDevice_RTMDELLINK_SignalsRecreate verifies a legitimate
// RTM_DELLINK for the watched interface still signals vxlanMissingChan.
func TestWatchVXLANDevice_RTMDELLINK_SignalsRecreate(t *testing.T) {
	nw := testNetwork("flannel.1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gotCh := make(chan chan<- netlink.LinkUpdate, 1)
	subscribe := func(ch chan<- netlink.LinkUpdate, done <-chan struct{}) error {
		gotCh <- ch
		return nil
	}

	vxlanMissingChan := make(chan bool, 1)
	stopped := runWatcher(nw, ctx, vxlanMissingChan, subscribe, 5*time.Millisecond)
	defer func() {
		cancel()
		requireClosedSoon(t, stopped, "watcher to exit")
	}()

	var ch chan<- netlink.LinkUpdate
	select {
	case ch = <-gotCh:
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for subscribe to be called")
	}

	ch <- netlink.LinkUpdate{
		Header: unix.NlMsghdr{Type: unix.RTM_DELLINK},
		Link:   &netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "flannel.1"}},
	}
	select {
	case missing, ok := <-vxlanMissingChan:
		if !ok || !missing {
			t.Fatalf("expected vxlanMissingChan to receive true, got value=%v ok=%v", missing, ok)
		}
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for vxlanMissingChan signal on matching RTM_DELLINK")
	}
}

// TestWatchVXLANDevice_NilLinkUpdate_NoPanic verifies a malformed update with
// a nil Link is skipped instead of panicking on Attrs().
func TestWatchVXLANDevice_NilLinkUpdate_NoPanic(t *testing.T) {
	nw := testNetwork("flannel.1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gotCh := make(chan chan<- netlink.LinkUpdate, 1)
	subscribe := func(ch chan<- netlink.LinkUpdate, done <-chan struct{}) error {
		gotCh <- ch
		return nil
	}

	vxlanMissingChan := make(chan bool, 1)
	stopped := runWatcher(nw, ctx, vxlanMissingChan, subscribe, 5*time.Millisecond)

	var ch chan<- netlink.LinkUpdate
	select {
	case ch = <-gotCh:
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for subscribe to be called")
	}

	ch <- netlink.LinkUpdate{} // zero value: Link is a nil interface

	// Confirm the watcher is still alive and processing updates afterwards.
	ch <- netlink.LinkUpdate{
		Header: unix.NlMsghdr{Type: unix.RTM_DELLINK},
		Link:   &netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "flannel.1"}},
	}
	select {
	case missing, ok := <-vxlanMissingChan:
		if !ok || !missing {
			t.Fatalf("expected vxlanMissingChan to receive true, got value=%v ok=%v", missing, ok)
		}
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for vxlanMissingChan signal after nil-Link update")
	}

	cancel()
	requireClosedSoon(t, stopped, "watcher to exit")
}
