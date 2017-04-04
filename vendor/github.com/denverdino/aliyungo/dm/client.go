package dm

import (
	"github.com/denverdino/aliyungo/common"
)

const (
	EmailEndPoint = "https://dm.aliyuncs.com/"
	SingleSendMail = "SingleSendMail"
	BatchSendMail = "BatchSendMail"
	EmailAPIVersion = "2015-11-23"
)

type Client struct {
	common.Client
}

func NewClient(accessKeyId, accessKeySecret string) *Client {
	client := &Client{}
	client.Init(EmailEndPoint, EmailAPIVersion, accessKeyId, accessKeySecret)
	return client
}
