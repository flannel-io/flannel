package sts

import (
	"os"

	"github.com/denverdino/aliyungo/common"
)

const (
	// STSDefaultEndpoint is the default API endpoint of STS services
	STSDefaultEndpoint = "https://sts.aliyuncs.com"
	STSAPIVersion      = "2015-04-01"
)

type STSClient struct {
	common.Client
}

func NewClient(accessKeyId string, accessKeySecret string) *STSClient {
	endpoint := os.Getenv("STS_ENDPOINT")
	if endpoint == "" {
		endpoint = STSDefaultEndpoint
	}
	return NewClientWithEndpoint(endpoint, accessKeyId, accessKeySecret)
}

func NewClientWithEndpoint(endpoint string, accessKeyId string, accessKeySecret string) *STSClient {
	client := &STSClient{}
	client.Init(endpoint, STSAPIVersion, accessKeyId, accessKeySecret)
	return client
}
