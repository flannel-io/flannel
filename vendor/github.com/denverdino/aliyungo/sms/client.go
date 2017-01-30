package sms

import "github.com/denverdino/aliyungo/common"

const (
	SmsEndPoint = "https://sms.aliyuncs.com/"
	SingleSendSms = "SingleSendSms"
	SmsAPIVersion = "2016-09-27"
)

type Client struct {
	common.Client
}

func NewClient(accessKeyId, accessKeySecret string) *Client {
	client := &Client{}
	client.Init(SmsEndPoint, SmsAPIVersion, accessKeyId, accessKeySecret)
	return client
}