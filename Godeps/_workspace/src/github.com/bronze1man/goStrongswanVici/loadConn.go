package goStrongswanVici

import (
	"fmt"
)

type Connection struct {
	ConnConf map[string]IKEConf `json:"connections"`
}

type IKEConf struct {
	LocalAddrs  []string `json:"local_addrs"`
	RemoteAddrs []string `json:"remote_addrs"`
	Proposals   []string `json:"proposals"` //aes128-sha256-modp1024
	Version     string   `json:"version"`   //1 for ikev1, 0 for ikev1 & ikev2
	Encap       string   `json:"encap"`     //yes,no
	KeyingTries string   `json:"keyingtries"`
	//	RekyTime   string                 `json:"rekey_time"`
	LocalAuth  AuthConf               `json:"local"`
	RemoteAuth AuthConf               `json:"remote"`
	Children   map[string]ChildSAConf `json:"children"`
}

type AuthConf struct {
	AuthMethod string `json:"auth"` //psk
}

type ChildSAConf struct {
	Local_ts      []string `json:"local_ts"`
	Remote_ts     []string `json:"remote_ts"`
	ESPProposals  []string `json:"esp_proposals"` //aes128-sha1_modp1024
	StartAction   string   `json:"start_action"`  //none,trap,start
	CloseAction   string   `json:"close_action"`
	ReqID         string   `json:"reqid"`
	RekeyTime     string   `json:"rekey_time"`
	Mode          string   `json:"mode"`
	InstallPolicy string   `json:"policies"`
}

func (c *ClientConn) LoadConn(conn *map[string]IKEConf) error {
	requestMap := &map[string]interface{}{}

	err := ConvertToGeneral(conn, requestMap)

	if err != nil {
		return fmt.Errorf("error creating request: %v", err)

	}
	msg, err := c.Request("load-conn", *requestMap)
	if msg["success"] != "yes" {
		return fmt.Errorf("unsuccessful LoadConn: %v", msg["success"])
	}

	return nil
}
