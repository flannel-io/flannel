// Copyright 2015 CoreOS, Inc.
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

package awsvpc

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/mitchellh/goamz/aws"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/mitchellh/goamz/ec2"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/pkg/task"
	"github.com/coreos/flannel/subnet"
)

type AwsVpcBackend struct {
	sm     *subnet.SubnetManager
	rawCfg json.RawMessage
	cfg    struct {
		RouteTableID string
	}
	stop chan bool
	wg   sync.WaitGroup
}

func New(sm *subnet.SubnetManager, config json.RawMessage) backend.Backend {
	be := AwsVpcBackend{
		sm:     sm,
		rawCfg: config,
		stop:   make(chan bool),
	}
	return &be
}

func (m *AwsVpcBackend) Init(extIface *net.Interface, extIP net.IP) (*backend.SubnetDef, error) {
	// Parse our configuration
	if len(m.rawCfg) > 0 {
		if err := json.Unmarshal(m.rawCfg, &m.cfg); err != nil {
			return nil, fmt.Errorf("error decoding VPC backend config: %v", err)
		}
	}

	// Acquire the lease form subnet manager
	attrs := subnet.LeaseAttrs{
		PublicIP: ip.FromIP(extIP),
	}

	sn, err := m.sm.AcquireLease(&attrs, m.stop)
	if err != nil {
		if err == task.ErrCanceled {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to acquire lease: %v", err)
		}
	}

	// Figure out this machine's EC2 instance ID and region
	identity, err := getInstanceIdentity()
	if err != nil {
		return nil, fmt.Errorf("error getting EC2 instance identity: %v", err)
	}

	instanceID, ok := identity["instanceId"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid EC2 instance ID: %v", identity["instanceId"])
	}

	regionVal, _ := identity["region"].(string)
	region, ok := aws.Regions[regionVal]
	if !ok {
		return nil, fmt.Errorf("invalid AWS region: %v", identity["region"])
	}

	// Setup the EC2 client
	auth, err := aws.GetAuth("", "")
	if err != nil {
		return nil, fmt.Errorf("error getting AWS credentials from environment: %v", err)
	}
	ec2c := ec2.New(auth, region)

	// Delete route for this machine's subnet if it already exists
	if _, err := ec2c.DeleteRoute(m.cfg.RouteTableID, sn.String()); err != nil {
		if ec2err, ok := err.(*ec2.Error); !ok || ec2err.Code != "InvalidRoute.NotFound" {
			// an error other than the route not already existing occurred
			return nil, fmt.Errorf("error deleting existing route for %s: %v", sn.String(), err)
		}
	}

	// Add the route for this machine's subnet
	route := &ec2.CreateRoute{
		RouteTableId:         m.cfg.RouteTableID,
		InstanceId:           instanceID,
		DestinationCidrBlock: sn.String(),
	}

	if _, err := ec2c.CreateRoute(route); err != nil {
		return nil, fmt.Errorf("unable to add route %+v: %v", route, err)
	}

	return &backend.SubnetDef{
		Net: sn,
		MTU: extIface.MTU,
	}, nil
}

func (m *AwsVpcBackend) Run() {
	m.sm.LeaseRenewer(m.stop)
}

func (m *AwsVpcBackend) Stop() {
	close(m.stop)
}

func (m *AwsVpcBackend) Name() string {
	return "aws-vpc"
}
