package network

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/backend/alloc"
	"github.com/coreos/flannel/backend/awsvpc"
	"github.com/coreos/flannel/backend/gce"
	"github.com/coreos/flannel/backend/hostgw"
	"github.com/coreos/flannel/backend/udp"
	"github.com/coreos/flannel/backend/vxlan"
	"github.com/coreos/flannel/subnet"
)

func newBackend(sm subnet.Manager, network string, config *subnet.Config) (backend.Backend, error) {
	var bt struct {
		Type string
	}

	if len(config.Backend) == 0 {
		bt.Type = "udp"
	} else {
		if err := json.Unmarshal(config.Backend, &bt); err != nil {
			return nil, fmt.Errorf("Error decoding Backend property of config: %v", err)
		}
	}

	switch strings.ToLower(bt.Type) {
	case "udp":
		return udp.New(sm, network, config), nil
	case "alloc":
		return alloc.New(sm, network), nil
	case "host-gw":
		return hostgw.New(sm, network), nil
	case "vxlan":
		return vxlan.New(sm, network, config), nil
	case "aws-vpc":
		return awsvpc.New(sm, network, config), nil
	case "gce":
		return gce.New(sm, network, config), nil
	default:
		return nil, fmt.Errorf("%v: '%v': unknown backend type", network, bt.Type)
	}
}
