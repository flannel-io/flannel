// Copyright 2015 flannel authors
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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

func init() {
	backend.Register("aws-vpc", New)
}

type AwsVpcBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := AwsVpcBackend{
		sm:       sm,
		extIface: extIface,
	}
	return &be, nil
}

func (be *AwsVpcBackend) Run(ctx context.Context) {
	<-ctx.Done()
}

func (be *AwsVpcBackend) RegisterNetwork(ctx context.Context, network string, config *subnet.Config) (backend.Network, error) {
	// Parse our configuration
	cfg := struct {
		RouteTableID string
	}{}

	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding VPC backend config: %v", err)
		}
	}

	// Acquire the lease form subnet manager
	attrs := subnet.LeaseAttrs{
		PublicIP: ip.FromIP(be.extIface.ExtAddr),
	}

	l, err := be.sm.AcquireLease(ctx, network, &attrs)
	switch err {
	case nil:

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	sess, _ := session.NewSession(aws.NewConfig().WithMaxRetries(5))

	// Figure out this machine's EC2 instance ID and region
	metadataClient := ec2metadata.New(sess)
	region, err := metadataClient.Region()
	if err != nil {
		return nil, fmt.Errorf("error getting EC2 region name: %v", err)
	}
	sess.Config.Region = aws.String(region)
	instanceID, err := metadataClient.GetMetadata("instance-id")
	if err != nil {
		return nil, fmt.Errorf("error getting EC2 instance ID: %v", err)
	}

	ec2c := ec2.New(sess)

	// Find ENI which contains the external network interface IP address
	eni, err := be.findENI(instanceID, ec2c)
	if err != nil || eni == nil {
		return nil, fmt.Errorf("unable to find ENI that matches the %s IP address. %s\n", be.extIface.IfaceAddr, err)
	}

	// Try to disable SourceDestCheck on the main network interface
	if err := be.disableSrcDestCheck(eni.NetworkInterfaceId, ec2c); err != nil {
		log.Warningf("failed to disable SourceDestCheck on %s: %s.\n", *eni.NetworkInterfaceId, err)
	}

	if cfg.RouteTableID == "" {
		if cfg.RouteTableID, err = be.detectRouteTableID(eni, ec2c); err != nil {
			return nil, err
		}
		log.Infof("Found route table %s.\n", cfg.RouteTableID)
	}

	networkConfig, err := be.sm.GetNetworkConfig(ctx, network)

	err = be.cleanupBlackholeRoutes(cfg.RouteTableID, networkConfig.Network, ec2c)
	if err != nil {
		log.Errorf("Error cleaning up blackhole routes: %v", err)
	}

	matchingRouteFound, err := be.checkMatchingRoutes(cfg.RouteTableID, l.Subnet.String(), eni.NetworkInterfaceId, ec2c)
	if err != nil {
		log.Errorf("Error describing route tables: %v", err)
	}

	if !matchingRouteFound {
		cidrBlock := l.Subnet.String()
		deleteRouteInput := &ec2.DeleteRouteInput{RouteTableId: &cfg.RouteTableID, DestinationCidrBlock: &cidrBlock}
		if _, err := ec2c.DeleteRoute(deleteRouteInput); err != nil {
			if ec2err, ok := err.(awserr.Error); !ok || ec2err.Code() != "InvalidRoute.NotFound" {
				// an error other than the route not already existing occurred
				return nil, fmt.Errorf("error deleting existing route for %s: %v", l.Subnet.String(), err)
			}
		}

		// Add the route for this machine's subnet
		if err := be.createRoute(cfg.RouteTableID, l.Subnet.String(), eni.NetworkInterfaceId, ec2c); err != nil {
			return nil, fmt.Errorf("unable to add route %s: %v", l.Subnet.String(), err)
		}
	}

	return &backend.SimpleNetwork{
		SubnetLease: l,
		ExtIface:    be.extIface,
	}, nil
}

func (be *AwsVpcBackend) cleanupBlackholeRoutes(routeTableID string, network ip.IP4Net, ec2c *ec2.EC2) error {
	filter := newFilter()
	filter.Add("route.state", "blackhole")

	input := ec2.DescribeRouteTablesInput{Filters: filter, RouteTableIds: []*string{&routeTableID}}
	resp, err := ec2c.DescribeRouteTables(&input)
	if err != nil {
		return err
	}

	for _, routeTable := range resp.RouteTables {
		for _, route := range routeTable.Routes {
			if *route.State == "blackhole" && route.DestinationCidrBlock != nil {
				_, subnet, err := net.ParseCIDR(*route.DestinationCidrBlock)
				if err == nil && network.Contains(ip.FromIP(subnet.IP)) {
					log.Info("Removing blackhole route: ", *route.DestinationCidrBlock)
					deleteRouteInput := &ec2.DeleteRouteInput{RouteTableId: &routeTableID, DestinationCidrBlock: route.DestinationCidrBlock}
					if _, err := ec2c.DeleteRoute(deleteRouteInput); err != nil {
						if ec2err, ok := err.(awserr.Error); !ok || ec2err.Code() != "InvalidRoute.NotFound" {
							// an error other than the route not already existing occurred
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func (be *AwsVpcBackend) checkMatchingRoutes(routeTableID, subnet string, eniID *string, ec2c *ec2.EC2) (bool, error) {
	matchingRouteFound := false

	filter := newFilter()
	filter.Add("route.destination-cidr-block", subnet)
	filter.Add("route.state", "active")

	input := ec2.DescribeRouteTablesInput{Filters: filter, RouteTableIds: []*string{&routeTableID}}

	resp, err := ec2c.DescribeRouteTables(&input)
	if err != nil {
		return matchingRouteFound, err
	}

	for _, routeTable := range resp.RouteTables {
		for _, route := range routeTable.Routes {
			if route.DestinationCidrBlock != nil && subnet == *route.DestinationCidrBlock &&
				*route.State == "active" && route.NetworkInterfaceId == eniID {
				matchingRouteFound = true
				break
			}
		}
	}

	return matchingRouteFound, nil
}

func (be *AwsVpcBackend) createRoute(routeTableID, subnet string, eniID *string, ec2c *ec2.EC2) error {
	route := &ec2.CreateRouteInput{
		RouteTableId:         &routeTableID,
		NetworkInterfaceId:   eniID,
		DestinationCidrBlock: &subnet,
	}

	if _, err := ec2c.CreateRoute(route); err != nil {
		return err
	}
	log.Infof("Route added %s - %s.\n", subnet, *eniID)
	return nil
}

func (be *AwsVpcBackend) disableSrcDestCheck(eniID *string, ec2c *ec2.EC2) error {
	attr := &ec2.ModifyNetworkInterfaceAttributeInput{
		NetworkInterfaceId: eniID,
		SourceDestCheck:    &ec2.AttributeBooleanValue{Value: aws.Bool(false)},
	}
	_, err := ec2c.ModifyNetworkInterfaceAttribute(attr)
	return err
}

// detectRouteTableID detect the routing table that is associated with the ENI,
// subnet can be implicitly associated with the main routing table
func (be *AwsVpcBackend) detectRouteTableID(eni *ec2.InstanceNetworkInterface, ec2c *ec2.EC2) (string, error) {
	subnetID := eni.SubnetId
	vpcID := eni.VpcId

	filter := newFilter()
	filter.Add("association.subnet-id", *subnetID)

	routeTablesInput := &ec2.DescribeRouteTablesInput{
		Filters: filter,
	}

	res, err := ec2c.DescribeRouteTables(routeTablesInput)
	if err != nil {
		return "", fmt.Errorf("error describing routeTables for subnetID %s: %v", *subnetID, err)
	}

	if len(res.RouteTables) != 0 {
		return *res.RouteTables[0].RouteTableId, nil
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
		return "", fmt.Errorf("main route table not found")
	}

	return *res.RouteTables[0].RouteTableId, nil
}

func (be *AwsVpcBackend) findENI(instanceID string, ec2c *ec2.EC2) (*ec2.InstanceNetworkInterface, error) {
	instance, err := ec2c.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)}},
	)
	if err != nil {
		return nil, err
	}

	for _, n := range instance.Reservations[0].Instances[0].NetworkInterfaces {
		for _, a := range n.PrivateIpAddresses {
			if *a.PrivateIpAddress == be.extIface.IfaceAddr.String() {
				log.Infof("Found %s that has %s IP address.\n", *n.NetworkInterfaceId, be.extIface.IfaceAddr)
				return n, nil
			}
		}
	}
	return nil, err
}
