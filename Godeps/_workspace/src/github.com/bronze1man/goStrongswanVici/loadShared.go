package goStrongswanVici

import (
	"fmt"
)

type Key struct {
	Typ    string   `json:"type"`
	Data   string   `json:"data"`
	Owners []string `json:"owners"`
}

func (c *ClientConn) LoadShared(key *Key) error {
	requestMap := &map[string]interface{}{}

	err := ConvertToGeneral(key, requestMap)

	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	msg, err := c.Request("load-shared", *requestMap)
	if msg["success"] != "yes" {
		return fmt.Errorf("unsuccessful loadSharedKey: %v", msg["success"])
	}

	return nil
}
