// Copyright 2014 Google Inc. All Rights Reserved.
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
	"fmt"
	"testing"
	"time"

	"google.golang.org/cloud"
	"google.golang.org/cloud/internal/testutil"
)

func TestAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}
	// TODO(djd): Replace this ctx with context.Background() when the new API is complete.
	ctx := testutil.Context(ScopePubSub, ScopeCloudPlatform)
	if ctx == nil {
		t.Skip("Integration tests skipped. See CONTRIBUTING.md for details")
	}
	ts := testutil.TokenSource(ctx, ScopePubSub, ScopeCloudPlatform)
	if ts == nil {
		t.Skip("Integration tests skipped. See CONTRIBUTING.md for details")
	}

	now := time.Now()
	topicName := fmt.Sprintf("topic-%d", now.Unix())
	subName := fmt.Sprintf("subscription-%d", now.Unix())

	client, err := NewClient(ctx, testutil.ProjID(), cloud.WithTokenSource(ts))
	if err != nil {
		t.Fatalf("Creating client error: %v", err)
	}

	var topic *TopicHandle
	if topic, err = client.NewTopic(ctx, topicName); err != nil {
		t.Errorf("CreateTopic error: %v", err)
	}

	var sub *SubscriptionHandle
	if sub, err = topic.Subscribe(ctx, subName, 0, nil); err != nil {
		t.Errorf("CreateSub error: %v", err)
	}

	exists, err := topic.Exists(ctx)
	if err != nil {
		t.Fatalf("TopicExists error: %v", err)
	}
	if !exists {
		t.Errorf("topic %s should exist, but it doesn't", topic)
	}

	exists, err = sub.Exists(ctx)
	if err != nil {
		t.Fatalf("SubExists error: %v", err)
	}
	if !exists {
		t.Errorf("subscription %s should exist, but it doesn't", subName)
	}

	max := 10
	msgs := make([]*Message, max)
	expectedMsgs := make(map[string]bool, max)
	for i := 0; i < max; i++ {
		text := fmt.Sprintf("a message with an index %d", i)
		attrs := make(map[string]string)
		attrs["foo"] = "bar"
		msgs[i] = &Message{
			Data:       []byte(text),
			Attributes: attrs,
		}
		expectedMsgs[text] = false
	}

	ids, err := Publish(ctx, topicName, msgs...)
	if err != nil {
		t.Fatalf("Publish (1) error: %v", err)
	}

	if len(ids) != max {
		t.Errorf("unexpected number of message IDs received; %d, want %d", len(ids), max)
	}

	expectedIDs := make(map[string]bool, max)
	for _, id := range ids {
		expectedIDs[id] = false
	}

	received, err := PullWait(ctx, subName, max)
	if err != nil {
		t.Fatalf("PullWait error: %v", err)
	}

	if len(received) != max {
		t.Errorf("unexpected number of messages received; %d, want %d", len(received), max)
	}

	for _, msg := range received {
		expectedMsgs[string(msg.Data)] = true
		expectedIDs[msg.ID] = true
		if msg.Attributes["foo"] != "bar" {
			t.Errorf("message attribute foo is expected to be 'bar', found '%s'", msg.Attributes["foo"])
		}
	}

	for msg, found := range expectedMsgs {
		if !found {
			t.Errorf("message '%s' should be received", msg)
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("message with the message id '%s' should be received", id)
		}
	}

	// base64 test
	data := "=@~"
	msg := &Message{
		Data: []byte(data),
	}
	_, err = Publish(ctx, topicName, msg)
	if err != nil {
		t.Fatalf("Publish (2) error: %v", err)
	}

	received, err = PullWait(ctx, subName, 1)
	if err != nil {
		t.Fatalf("PullWait error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("unexpected number of messages received; %d, want %d", len(received), 1)
	}
	if string(received[0].Data) != data {
		t.Errorf("unexpexted message received; %s, want %s", string(received[0].Data), data)
	}

	err = sub.Delete(ctx)
	if err != nil {
		t.Errorf("DeleteSub error: %v", err)
	}

	err = topic.Delete(ctx)
	if err != nil {
		t.Errorf("DeleteTopic error: %v", err)
	}
}
