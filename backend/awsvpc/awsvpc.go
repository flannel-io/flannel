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
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/mitchellh/goamz/aws"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/mitchellh/goamz/ec2"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
	"net"
	"sync"
)

type AwsVpcBackend struct {
	sm      subnet.Manager
	network string
	config  *subnet.Config
	cfg     struct {
		RouteTableID string
	}
	lease  *subnet.Lease
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(sm subnet.Manager, network string, config *subnet.Config) backend.Backend {
	ctx, cancel := context.WithCancel(context.Background())

	be := AwsVpcBackend{
		sm:      sm,
		network: network,
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
	}
	return &be
}

func (m *AwsVpcBackend) Init(extIface *net.Interface, extIP net.IP) (*backend.SubnetDef, error) {
	// Parse our configuration
	if len(m.config.Backend) > 0 {
		if err := json.Unmarshal(m.config.Backend, &m.cfg); err != nil {
			return nil, fmt.Errorf("error decoding VPC backend config: %v", err)
		}
	}

	// Acquire the lease form subnet manager
	attrs := subnet.LeaseAttrs{
		PublicIP: ip.FromIP(extIP),
	}

	l, err := m.sm.AcquireLease(m.ctx, m.network, &attrs)
	switch err {
	case nil:
		m.lease = l

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
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

	if _, err = m.disableSrcDestCheck(instanceID, ec2c); err != nil {
		log.Info("Warning- disabling source destination check falied!: %v", err)
	}

	if m.cfg.RouteTableID == "" {
		log.Infof("RouteTableID not passed as config parameter, attempting to detect")
		routeTableID, err := m.DetectRouteTableID(instanceID, ec2c)
		if err != nil {
			return nil, err
		}
		log.Info("Detected routeRouteTableID: ", routeTableID)

		m.cfg.RouteTableID = routeTableID
	}

	filter := ec2.NewFilter()
	filter.Add("route.destination-cidr-block", l.Subnet.String())
	filter.Add("route.state", "active")

	resp, err := ec2c.DescribeRouteTables([]string{m.cfg.RouteTableID}, filter)
	if err != nil {
		log.Errorf("Error describing route tables: %v", err)

		if ec2Err, ok := err.(*ec2.Error); ok {
			if ec2Err.Code == "UnauthorizedOperation" {
				log.Errorf("Note: describeRouteTables permission cannot be bound to any resource")
			}
		}

	} else {
		for _, routeTable := range resp.RouteTables {
			for _, route := range routeTable.Routes {
				if l.Subnet.String() == route.DestinationCidrBlock && route.State == "active" {
					log.Errorf("Matching *active* entry to: %s that will be deleted: %s, %s \n", l.Subnet.String(), route.DestinationCidrBlock, route.GatewayId)
				}
			}
		}
	}

	// Delete route for this machine's subnet if it already exists
	if _, err := ec2c.DeleteRoute(m.cfg.RouteTableID, l.Subnet.String()); err != nil {
		if ec2err, ok := err.(*ec2.Error); !ok || ec2err.Code != "InvalidRoute.NotFound" {
			// an error other than the route not already existing occurred
			return nil, fmt.Errorf("error deleting existing route for %s: %v", l.Subnet.String(), err)
		}
	}

	// Add the route for this machine's subnet
	route := &ec2.CreateRoute{
		RouteTableId:         m.cfg.RouteTableID,
		InstanceId:           instanceID,
		DestinationCidrBlock: l.Subnet.String(),
	}

	if _, err := ec2c.CreateRoute(route); err != nil {
		return nil, fmt.Errorf("unable to add route %+v: %v", route, err)
	}

	return &backend.SubnetDef{
		Net: l.Subnet,
		MTU: extIface.MTU,
	}, nil
}

func (m *AwsVpcBackend) disableSrcDestCheck(instanceID string, ec2c *ec2.EC2) (*ec2.ModifyInstanceResp, error) {
	var modifyInstance ec2.ModifyInstance
	modifyInstance.SourceDestCheck = false
	modifyInstance.SetSourceDestCheck = true

	return ec2c.ModifyInstance(instanceID, &modifyInstance)
}

func (m *AwsVpcBackend) DetectRouteTableID(instanceID string, ec2c *ec2.EC2) (string, error) {
	resp, err := ec2c.Instances([]string{instanceID}, nil)
	if err != nil {
		return "", fmt.Errorf("error getting instance info: %v", err)
	}

	subnetID := resp.Reservations[0].Instances[0].SubnetId
	log.Info("SubnetId: ", subnetID)

	filter := ec2.NewFilter()
	filter.Add("association.subnet-id", subnetID)

	res, err := ec2c.DescribeRouteTables(nil, filter)
	if err != nil {
		return "", fmt.Errorf("error describing routeTables for subnetID %s: %v", subnetID, err)
	}

	return res.RouteTables[0].RouteTableId, nil
}

func (m *AwsVpcBackend) Run() {
	subnet.LeaseRenewer(m.ctx, m.sm, m.network, m.lease)
}

func (m *AwsVpcBackend) Stop() {
	m.cancel()
}

func (m *AwsVpcBackend) Name() string {
	return "aws-vpc"
}
