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

package alivpc

import (
	"encoding/json"
	"fmt"
	"os"

	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
	"github.com/denverdino/aliyungo/common"
	"github.com/denverdino/aliyungo/ecs"
	"github.com/denverdino/aliyungo/metadata"
)

func init() {
	backend.Register("ali-vpc", New)
}

type AliVpcBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := AliVpcBackend{
		sm:       sm,
		extIface: extIface,
	}
	return &be, nil
}

func (be *AliVpcBackend) Run(ctx context.Context) {
	<-ctx.Done()
}

func (be *AliVpcBackend) RegisterNetwork(ctx context.Context, config *subnet.Config) (backend.Network, error) {
	// 1. Parse our configuration
	cfg := struct {
		AccessKeyID     string
		AccessKeySecret string
	}{}

	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding VPC backend config: %v", err)
		}
	}
	log.Infof("Unmarshal Configure : %v\n", cfg)

	// 2. Acquire the lease form subnet manager
	attrs := subnet.LeaseAttrs{
		PublicIP: ip.FromIP(be.extIface.ExtAddr),
	}

	l, err := be.sm.AcquireLease(ctx, &attrs)
	switch err {
	case nil:

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}
	if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		cfg.AccessKeyID = os.Getenv("ACCESS_KEY_ID")
		cfg.AccessKeySecret = os.Getenv("ACCESS_KEY_SECRET")

		if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
			return nil, fmt.Errorf("ACCESS_KEY_ID and ACCESS_KEY_SECRET must be provided! ")
		}
	}

	meta := metadata.NewMetaData(nil)
	REGION, err := meta.Region()
	if err != nil {
		return nil, err
	}
	instanceid, err := meta.InstanceID()
	if err != nil {
		return nil, err
	}
	VpcID, err := meta.VpcID()
	if err != nil {
		return nil, err
	}

	c := ecs.NewClient(cfg.AccessKeyID, cfg.AccessKeySecret)

	vpc, _, err := c.DescribeVpcs(&ecs.DescribeVpcsArgs{
		RegionId: common.Region(REGION),
		VpcId:    VpcID,
	})
	if err != nil || len(vpc) <= 0 {
		log.Errorf("Error DescribeVpcs: %s . \n", getErrorString(err))
		return nil, err
	}

	vroute, _, err := c.DescribeVRouters(&ecs.DescribeVRoutersArgs{
		VRouterId: vpc[0].VRouterId,
		RegionId:  common.Region(REGION),
	})
	if err != nil || len(vroute) <= 0 {
		log.Errorf("Error DescribeVRouters: %s .\n", getErrorString(err))
		return nil, err
	}
	vRouterId := vroute[0].VRouterId
	rTableId := vroute[0].RouteTableIds.RouteTableId[0]

	rtables, _, err := c.DescribeRouteTables(&ecs.DescribeRouteTablesArgs{
		VRouterId:    vRouterId,
		RouteTableId: rTableId,
	})
	if err != nil || len(rtables) <= 0 {
		log.Errorf("Error DescribeRouteTables: %s.\n", err.Error())
		return nil, err
	}

	route := &ecs.CreateRouteEntryArgs{
		DestinationCidrBlock: l.Subnet.String(),
		NextHopType:          ecs.NextHopIntance,
		NextHopId:            instanceid,
		ClientToken:          "",
		RouteTableId:         rTableId,
	}
	if err := be.recreateRoute(c, rtables[0], route); err != nil {
		return nil, err
	}

	if err := c.WaitForAllRouteEntriesAvailable(vRouterId, rTableId, 60); err != nil {
		return nil, err
	}
	return &backend.SimpleNetwork{
		SubnetLease: l,
		ExtIface:    be.extIface,
	}, nil
}

func (be *AliVpcBackend) recreateRoute(c *ecs.Client, table ecs.RouteTableSetType, route *ecs.CreateRouteEntryArgs) error {
	exist := false
	for _, e := range table.RouteEntrys.RouteEntry {
		if e.RouteTableId == route.RouteTableId &&
			e.Type == ecs.RouteTableCustom &&
			e.InstanceId == route.NextHopId {

			if e.DestinationCidrBlock == route.DestinationCidrBlock &&
				e.Status == ecs.RouteEntryStatusAvailable {
				exist = true
				log.Infof("Keep target entry: rtableid=%s, CIDR=%s, NextHop=%s \n", e.RouteTableId, e.DestinationCidrBlock, e.InstanceId)
				continue
			}
			// Fix: here we delete all the route which targeted to us(instance) except the specified route.
			// That means only one CIDR was allowed to target to the instance. Think if We need to change this
			// to adapt to multi CIDR and deal with unavailable route entry.
			if err := c.DeleteRouteEntry(&ecs.DeleteRouteEntryArgs{
				RouteTableId:         route.RouteTableId,
				DestinationCidrBlock: e.DestinationCidrBlock,
				NextHopId:            route.NextHopId,
			}); err != nil {
				return err
			}

			log.Infof("Remove old route entry: rtableid=%s, CIDR=%s, NextHop=%s \n", e.RouteTableId, e.DestinationCidrBlock, e.InstanceId)
			continue
		}

		log.Infof("Keep route entry: rtableid=%s, CIDR=%s, NextHop=%s \n", e.RouteTableId, e.DestinationCidrBlock, e.InstanceId)
	}
	if !exist {
		return c.CreateRouteEntry(route)
	}
	return nil
}

func getErrorString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}
