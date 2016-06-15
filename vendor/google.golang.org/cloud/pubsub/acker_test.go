// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pubsub

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestAcker(t *testing.T) {
	tick := make(chan time.Time)
	s := &testService{acknowledgeCalled: make(chan acknowledgeCall)}
	c := &Client{projectID: "projid", s: s}

	processed := make(chan string, 10)
	acker := &acker{
		Client:  c,
		Ctx:     context.Background(),
		Sub:     "subname",
		AckTick: tick,
		Notify:  func(ackID string) { processed <- ackID },
	}
	acker.Start()

	checkAckProcessed := func(ackIDs []string) {
		got := <-s.acknowledgeCalled
		sort.Strings(got.ackIDs)

		want := acknowledgeCall{
			subName: "subname",
			ackIDs:  ackIDs,
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("acknowledge: got:\n%v\nwant:\n%v", got, want)
		}
	}

	acker.Ack("a")
	acker.Ack("b")
	tick <- time.Time{}
	checkAckProcessed([]string{"a", "b"})
	acker.Ack("c")
	tick <- time.Time{}
	checkAckProcessed([]string{"c"})
	acker.Stop()

	// all IDS should have been sent to processed.
	close(processed)
	processedIDs := []string{}
	for id := range processed {
		processedIDs = append(processedIDs, id)
	}
	sort.Strings(processedIDs)
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(processedIDs, want) {
		t.Errorf("acker processed: got:\n%v\nwant:\n%v", processedIDs, want)
	}
}

// TestAckerStop checks that Stop blocks until all ackIDs have been acked.
func TestAckerStop(t *testing.T) {
	tick := make(chan time.Time)
	s := &testService{acknowledgeCalled: make(chan acknowledgeCall, 10)}
	c := &Client{projectID: "projid", s: s}

	processed := make(chan string)
	acker := &acker{
		Client:  c,
		Ctx:     context.Background(),
		Sub:     "subname",
		AckTick: tick,
		Notify:  func(ackID string) { processed <- ackID },
	}

	acker.Start()

	stopped := make(chan struct{})

	// Add an ackID so that acker.Stop will not return immediately.
	acker.Ack("a")

	go func() {
		acker.Stop()
		stopped <- struct{}{}
	}()

	// If acker,Stop fails to block, stopped should have been written to by the time
	// this sleep completes.
	time.Sleep(time.Millisecond)

	// Receiving from processed should cause Stop to subsequently return,
	// so it should never be possible to read from stopped before
	// processed.
	select {
	case <-processed:
	case <-stopped:
		t.Errorf("acker.Stop returned before cleanup was complete")
	case <-time.After(time.Millisecond):
		t.Errorf("send to processed never arrived")
	}
	select {
	case <-stopped:
	case <-time.After(time.Millisecond):
		t.Errorf("acker.Stop never returned")
	}
}
