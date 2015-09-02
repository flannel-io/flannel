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

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/aws/aws-sdk-go/aws"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

type AwsVpcBackend struct {
	sm       subnet.Manager
	publicIP ip.IP4
	mtu      int
	cfg      struct {
		 RouteTableID string
	}
	lease    *subnet.Lease
}

func New(sm subnet.Manager, extIface *net.Interface, extIaddr net.IP, extEaddr net.IP) (backend.Backend, error) {
	be := AwsVpcBackend{
		sm:      sm,
		publicIP: ip.FromIP(extEaddr),
		mtu:      extIface.MTU,
	}
	return &be, nil
}

func (m *AwsVpcBackend) RegisterNetwork(ctx context.Context, network string, config *subnet.Config) (*backend.SubnetDef, error) {
	// Parse our configuration
	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &m.cfg); err != nil {
			return nil, fmt.Errorf("error decoding VPC backend config: %v", err)
		}
	}

	// Acquire the lease form subnet manager
	attrs := subnet.LeaseAttrs{
		PublicIP: m.publicIP,
	}

	l, err := m.sm.AcquireLease(ctx, network, &attrs)
	switch err {
	case nil:
		m.lease = l

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	// Figure out this machine's EC2 instance ID and region
	metadataClient := ec2metadata.New(nil)
	region, err := metadataClient.Region()
	if err != nil {
		return nil, fmt.Errorf("error getting EC2 region name: %v", err)
	}
	instanceID, err := metadataClient.GetMetadata("instance-id")
	if err != nil {
		return nil, fmt.Errorf("error getting EC2 instance ID: %v", err)
	}

	ec2c := ec2.New(&aws.Config{Region: aws.String(region)})

	if _, err = m.disableSrcDestCheck(instanceID, ec2c); err != nil {
		log.Infof("Warning- disabling source destination check failed: %v", err)
	}

	if m.cfg.RouteTableID == "" {
		log.Infof("RouteTableID not passed as config parameter, detecting ...")
		if err := m.detectRouteTableID(instanceID, ec2c); err != nil {
			return nil, err
		}
	}

	log.Info("RouteRouteTableID: ", m.cfg.RouteTableID)

	matchingRouteFound, err := m.checkMatchingRoutes(instanceID, l.Subnet.String(), ec2c)
	if err != nil {
		log.Errorf("Error describing route tables: %v", err)

		if ec2Err, ok := err.(awserr.Error); ok {
			if ec2Err.Code() == "UnauthorizedOperation" {
				log.Errorf("Note: DescribeRouteTables permission cannot be bound to any resource")
			}
		}
	}

	if !matchingRouteFound {
		cidrBlock := l.Subnet.String()
		deleteRouteInput := &ec2.DeleteRouteInput{RouteTableId: &m.cfg.RouteTableID, DestinationCidrBlock: &cidrBlock}
		if _, err := ec2c.DeleteRoute(deleteRouteInput); err != nil {
			if ec2err, ok := err.(awserr.Error); !ok || ec2err.Code() != "InvalidRoute.NotFound" {
				// an error other than the route not already existing occurred
				return nil, fmt.Errorf("error deleting existing route for %s: %v", l.Subnet.String(), err)
			}
		}

		// Add the route for this machine's subnet
		if _, err := m.createRoute(instanceID, l.Subnet.String(), ec2c); err != nil {
			return nil, fmt.Errorf("unable to add route %s: %v", l.Subnet.String(), err)
		}
	}

	return &backend.SubnetDef{
		Lease: l,
		MTU:   m.mtu,
	}, nil
}

func (m *AwsVpcBackend) checkMatchingRoutes(instanceID, subnet string, ec2c *ec2.EC2) (bool, error) {
	matchingRouteFound := false

	filter := newFilter()
	filter.Add("route.destination-cidr-block", subnet)
	filter.Add("route.state", "active")

	input := ec2.DescribeRouteTablesInput{Filters: filter, RouteTableIds: []*string{&m.cfg.RouteTableID}}

	resp, err := ec2c.DescribeRouteTables(&input)
	if err != nil {
		return matchingRouteFound, err
	}

	for _, routeTable := range resp.RouteTables {
		for _, route := range routeTable.Routes {
			if subnet == *route.DestinationCidrBlock && *route.State == "active" {

				if *route.InstanceId == instanceID {
					matchingRouteFound = true
					break
				}

				log.Errorf("Deleting invalid *active* matching route: %s, %s \n", *route.DestinationCidrBlock, *route.InstanceId)
			}
		}
	}

	return matchingRouteFound, nil
}

func (m *AwsVpcBackend) createRoute(instanceID, subnet string, ec2c *ec2.EC2) (*ec2.CreateRouteOutput, error) {
	route := &ec2.CreateRouteInput{
		RouteTableId:         &m.cfg.RouteTableID,
		InstanceId:           &instanceID,
		DestinationCidrBlock: &subnet,
	}

	return ec2c.CreateRoute(route)
}
func (m *AwsVpcBackend) disableSrcDestCheck(instanceID string, ec2c *ec2.EC2) (*ec2.ModifyInstanceAttributeOutput, error) {
	modifyAttributes := &ec2.ModifyInstanceAttributeInput{
		InstanceId:      aws.String(instanceID),
		SourceDestCheck: &ec2.AttributeBooleanValue{Value: aws.Bool(false)},
	}

	return ec2c.ModifyInstanceAttribute(modifyAttributes)
}

func (m *AwsVpcBackend) detectRouteTableID(instanceID string, ec2c *ec2.EC2) error {
	instancesInput := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{&instanceID},
	}

	resp, err := ec2c.DescribeInstances(instancesInput)
	if err != nil {
		return fmt.Errorf("error getting instance info: %v", err)
	}

	if len(resp.Reservations) == 0 {
		return fmt.Errorf("no reservations found")
	}

	if len(resp.Reservations[0].Instances) == 0 {
		return fmt.Errorf("no matching instance found with id: %v", instanceID)
	}

	subnetID := resp.Reservations[0].Instances[0].SubnetId
	vpcID := resp.Reservations[0].Instances[0].VpcId

	log.Info("Subnet-ID: ", *subnetID)
	log.Info("VPC-ID: ", *vpcID)

	filter := newFilter()
	filter.Add("association.subnet-id", *subnetID)

	routeTablesInput := &ec2.DescribeRouteTablesInput{
		Filters: filter,
	}

	res, err := ec2c.DescribeRouteTables(routeTablesInput)
	if err != nil {
		return fmt.Errorf("error describing routeTables for subnetID %s: %v", *subnetID, err)
	}

	if len(res.RouteTables) != 0 {
		m.cfg.RouteTableID = *res.RouteTables[0].RouteTableId
		return nil
	}

	filter = newFilter()
	filter.Add("association.main", "true")
	filter.Add("vpc-id", *vpcID)

	routeTablesInput = &ec2.DescribeRouteTablesInput{
		Filters: filter,
	}

	res, err = ec2c.DescribeRouteTables(routeTablesInput)
	if err != nil {
		log.Info("error describing route tables: ", err)
	}

	if len(res.RouteTables) == 0 {
		return fmt.Errorf("main route table not found")
	}

	m.cfg.RouteTableID = *res.RouteTables[0].RouteTableId

	return nil
}

func (m *AwsVpcBackend) Run(ctx context.Context) {
}
