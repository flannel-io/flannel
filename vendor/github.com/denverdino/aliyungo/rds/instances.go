package rds

import "github.com/denverdino/aliyungo/common"

// ref: https://help.aliyun.com/document_detail/26242.html
type ModifySecurityIpsArgs struct {
	DBInstanceId               string
	SecurityIps                string
	DBInstanceIPArrayName      string
	DBInstanceIPArrayAttribute string
}

func (client *Client) ModifySecurityIps(args *ModifySecurityIpsArgs) (resp common.Response, err error) {
	response := common.Response{}
	err = client.Invoke("ModifySecurityIps", args, &response)
	return response, err
}
