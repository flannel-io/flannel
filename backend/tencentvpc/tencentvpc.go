// Copyright 2015 flannel authors
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
// +build !windows

package tencentvpc

import (
	"encoding/json"
	"fmt"
	"github.com/flannel-io/flannel/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/subnet"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"golang.org/x/net/context"
	"io/ioutil"
	log "k8s.io/klog"
	"net/http"
	"os"
	"sync"
)

func init() {
	backend.Register("tencent-vpc", New)
}

type TencentVpcBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := TencentVpcBackend{
		sm:       sm,
		extIface: extIface,
	}
	return &be, nil
}

func get_vm_metadata(url string) (string, error) {
	resp, err := http.Get(url)

	if err != nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get vm region error: %v", err)
	}

	defer resp.Body.Close()

	metadata, _ := ioutil.ReadAll(resp.Body)
	return string(metadata), nil
}

func get_vm_region() (string, error) {
	url := "http://metadata.tencentyun.com/latest/meta-data/placement/region"
	return get_vm_metadata(url)
}

func get_vm_vpcid() (string, error) {
	macUrl := "http://metadata.tencentyun.com/latest/meta-data/mac"
	mac, err := get_vm_metadata(macUrl)

	if err != nil {
		return "", fmt.Errorf("get vm mac error: %v", err)
	}

	vpcUrl := fmt.Sprintf("http://metadata.tencentyun.com/latest/meta-data/network/interfaces/macs/%s/vpc-id", mac)
	vpcid, err := get_vm_metadata(vpcUrl)

	if err != nil {
		return "", fmt.Errorf("get vm vpcid error: %v", err)
	}

	return vpcid, nil
}

func (be *TencentVpcBackend) RegisterNetwork(ctx context.Context, wg *sync.WaitGroup, config *subnet.Config) (backend.Network, error) {
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

	region, err := get_vm_region()
	if err != nil {
		return nil, err
	}
	vpcid, err := get_vm_vpcid()
	if err != nil {
		return nil, err
	}

	c, _ := vpc.NewClientWithSecretId(cfg.AccessKeyID, cfg.AccessKeySecret, region)
	request := vpc.NewDescribeRouteTablesRequest()
	request.Filters = []*vpc.Filter{
		&vpc.Filter{
			Name:   common.StringPtr("vpc-id"),
			Values: common.StringPtrs([]string{vpcid}),
		},
	}

	res, err := c.DescribeRouteTables(request)
	if _, ok := err.(*errors.TencentCloudSDKError); ok {
		return nil, fmt.Errorf("describe route table error: %v", ok)
	}

	if err != nil {
		return nil, err
	}

	response := res.Response

	if len(response.RouteTableSet) <= 0 {
		return nil, fmt.Errorf("No suitable routing table found")
	}

	routeTable := response.RouteTableSet[0]
	exists := false
	gatewayType := "NORMAL_CVM"
	routeType := "USER"

	for _, route := range routeTable.RouteSet {
		if *route.DestinationCidrBlock == l.Subnet.String() &&
			*route.GatewayId == be.extIface.ExtAddr.String() &&
			*route.GatewayType == gatewayType &&
			*route.RouteType == routeType {
			if *route.Enabled {
				exists = true
			} else {
				delRouteRequest := vpc.NewDeleteRoutesRequest()
				delRouteRequest.RouteTableId = routeTable.RouteTableId
				delRouteRequest.Routes = []*vpc.Route{
					&vpc.Route{
						RouteId: route.RouteId,
					},
				}

				_, err := c.DeleteRoutes(delRouteRequest)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if !exists {
		createRouteRequest := vpc.NewCreateRoutesRequest()
		createRouteRequest.RouteTableId = routeTable.RouteTableId
		createRouteRequest.Routes = []*vpc.Route{
			&vpc.Route{
				DestinationCidrBlock: common.StringPtr(l.Subnet.String()),
				GatewayType:          &gatewayType,
				GatewayId:            common.StringPtr(be.extIface.ExtAddr.String()),
				Enabled:              common.BoolPtr(true),
			},
		}

		_, err := c.CreateRoutes(createRouteRequest)

		if err != nil {
			return nil, err
		}
	}

	return &backend.SimpleNetwork{
		SubnetLease: l,
		ExtIface:    be.extIface,
	}, nil
}
