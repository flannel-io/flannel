package cms

import (
	"net/http"
)

type Client struct {
	endpoint        string
	accessKeyId     string //Access Key Id
	accessKeySecret string //Access Key Secret

	debug      bool
	httpClient *http.Client
	version    string
	internal   bool
	//region     common.Region
}

const (
	DefaultEndpoint = "http://alert.aliyuncs.com"
	APIVersion      = "2015-08-15"
	METHOD_GET      = "GET"
	METHOD_POST     = "POST"
	METHOD_PUT      = "PUT"
	METHOD_DELETE   = "DELETE"
)

// NewClient creates a new instance of ECS client
func NewClient(accessKeyId, accessKeySecret string) *Client {
	return &Client{
		accessKeyId:     accessKeyId,
		accessKeySecret: accessKeySecret,
		internal:        false,
		//region:          region,
		version:    APIVersion,
		endpoint:   DefaultEndpoint,
		httpClient: &http.Client{},
	}
}

func (client *Client) GetApiUri() string {
	return client.endpoint
}

func (client *Client) GetAccessKey() string {
	return client.accessKeyId
}

func (client *Client) GetAccessSecret() string {
	return client.accessKeySecret
}
