package push

import "github.com/denverdino/aliyungo/common"

const (
	PushEndPoint = "https://cloudpush.aliyuncs.com/"
	Push = "Push"
	PushAPIVersion = "2015-08-27"
)

type Client struct {
	common.Client
}

func NewClient(accessKeyId, accessKeySecret string) *Client {
	client := &Client{}
	client.Init(PushEndPoint, PushAPIVersion, accessKeyId, accessKeySecret)
	return client
}