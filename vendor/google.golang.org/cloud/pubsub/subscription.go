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
	"fmt"
	"time"

	"golang.org/x/net/context"
)

// SubscriptionHandle is a reference to a PubSub subscription.
type SubscriptionHandle struct {
	c *Client

	// The fully qualified identifier for the subscription, in the format "projects/<projid>/subscriptions/<name>"
	name string
}

// Subscription creates a reference to a subscription.
func (c *Client) Subscription(name string) *SubscriptionHandle {
	return &SubscriptionHandle{
		c:    c,
		name: fmt.Sprintf("projects/%s/subscriptions/%s", c.projectID, name),
	}
}

// Name returns the globally unique name for the subscription.
func (s *SubscriptionHandle) Name() string {
	return s.name
}

// Subscriptions lists all of the subscriptions for the client's project.
func (c *Client) Subscriptions(ctx context.Context) ([]*SubscriptionHandle, error) {
	subNames, err := c.s.listProjectSubscriptions(ctx, c.fullyQualifiedProjectName())
	if err != nil {
		return nil, err
	}

	subs := []*SubscriptionHandle{}
	for _, s := range subNames {
		subs = append(subs, &SubscriptionHandle{c: c, name: s})
	}
	return subs, nil
}

// PushConfig contains configuration for subscriptions that operate in push mode.
type PushConfig struct {
	// A URL locating the endpoint to which messages should be pushed.
	Endpoint string

	// Endpoint configuration attributes. See https://cloud.google.com/pubsub/reference/rest/v1/projects.subscriptions#PushConfig.FIELDS.attributes for more details.
	Attributes map[string]string
}

// Subscription config contains the configuration of a subscription.
type SubscriptionConfig struct {
	Topic      *TopicHandle
	PushConfig PushConfig

	// The default maximum time after a subscriber receives a message
	// before the subscriber should acknowledge the message.  Note:
	// messages which are obtained via an Iterator need not be acknowledged
	// within this deadline, as the deadline will be automatically
	// extended.
	AckDeadline time.Duration
}

// Delete deletes the subscription.
func (s *SubscriptionHandle) Delete(ctx context.Context) error {
	return s.c.s.deleteSubscription(ctx, s.name)
}

// Exists reports whether the subscription exists on the server.
func (s *SubscriptionHandle) Exists(ctx context.Context) (bool, error) {
	return s.c.s.subscriptionExists(ctx, s.name)
}

// Config fetches the current configuration for the subscription.
func (s *SubscriptionHandle) Config(ctx context.Context) (*SubscriptionConfig, error) {
	sub, topicName, err := s.c.s.getSubscriptionConfig(ctx, s.name)
	if err != nil {
		return nil, err
	}
	sub.Topic = &TopicHandle{
		c:    s.c,
		name: topicName,
	}
	return sub, nil
}

// Pull returns an Iterator that can be used to fetch Messages.
// The Iterator will automatically extend the ack deadline of all fetched
// Messages, for the period specified maxExtension. Automatic deadline
// extension may be disabled by specifying a maxExtension of 0.
//
// The caller must call Close on the Iterator once finished with it.
func (s *SubscriptionHandle) Pull(ctx context.Context, maxExtension time.Duration) (*Iterator, error) {
	// TODO(mcgreevy): accept pulloptions.
	config, err := s.Config(ctx)
	if err != nil {
		return nil, err
	}
	return s.c.newIterator(ctx, s.name, config.AckDeadline, maxExtension), nil
}
