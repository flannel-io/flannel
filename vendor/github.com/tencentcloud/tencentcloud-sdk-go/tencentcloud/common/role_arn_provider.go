package common

import (
	"encoding/json"
	"strconv"
	"time"

	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
)

const (
	endpoint               = "sts.tencentcloudapi.com"
	service                = "sts"
	version                = "2018-08-13"
	region                 = regions.Guangzhou
	defaultSessionName     = "tencentcloud-go-sdk-"
	action                 = "AssumeRole"
	defaultDurationSeconds = 7200
)

type RoleArnProvider struct {
	longSecretId    string
	longSecretKey   string
	roleArn         string
	roleSessionName string
	durationSeconds int64
}

type stsRsp struct {
	Response struct {
		Credentials struct {
			Token        string `json:"Token"`
			TmpSecretId  string `json:"TmpSecretId"`
			TmpSecretKey string `json:"TmpSecretKey"`
		} `json:"Credentials"`
		ExpiredTime int       `json:"ExpiredTime"`
		Expiration  time.Time `json:"Expiration"`
		RequestId   string    `json:"RequestId"`
	} `json:"Response"`
}

func NewRoleArnProvider(secretId, secretKey, roleArn, sessionName string, duration int64) *RoleArnProvider {
	return &RoleArnProvider{
		longSecretId:    secretId,
		longSecretKey:   secretKey,
		roleArn:         roleArn,
		roleSessionName: sessionName,
		durationSeconds: duration,
	}
}

// DefaultRoleArnProvider returns a RoleArnProvider that use some default options:
//  1. roleSessionName will be "tencentcloud-go-sdk-" + timestamp
//  2. durationSeconds will be 7200s
func DefaultRoleArnProvider(secretId, secretKey, roleArn string) *RoleArnProvider {
	return NewRoleArnProvider(secretId, secretKey, roleArn, defaultSessionName+strconv.FormatInt(time.Now().UnixNano()/1000, 10), defaultDurationSeconds)
}

func (r *RoleArnProvider) GetCredential() (CredentialIface, error) {
	if r.durationSeconds > 43200 || r.durationSeconds <= 0 {
		return nil, tcerr.NewTencentCloudSDKError(creErr, "Assume Role durationSeconds should be in the range of 0~43200s", "")
	}
	credential := NewCredential(r.longSecretId, r.longSecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = endpoint
	cpf.HttpProfile.ReqMethod = "POST"

	client := NewCommonClient(credential, region, cpf)
	request := tchttp.NewCommonRequest(service, version, action)

	params := map[string]interface{}{
		"RoleArn":         r.roleArn,
		"RoleSessionName": r.roleSessionName,
		"DurationSeconds": r.durationSeconds,
	}
	err := request.SetActionParameters(params)
	if err != nil {
		return nil, err
	}

	response := tchttp.NewCommonResponse()
	err = client.Send(request, response)
	if err != nil {
		return nil, err
	}
	rspSt := new(stsRsp)

	if err = json.Unmarshal(response.GetBody(), rspSt); err != nil {
		return nil, tcerr.NewTencentCloudSDKError(creErr, err.Error(), "")
	}

	return &RoleArnCredential{
		roleArn:         r.roleArn,
		roleSessionName: r.roleSessionName,
		durationSeconds: r.durationSeconds,
		expiredTime:     int64(rspSt.Response.ExpiredTime) - r.durationSeconds/10*9, // credential's actual duration time is 1/10 of the original
		token:           rspSt.Response.Credentials.Token,
		tmpSecretId:     rspSt.Response.Credentials.TmpSecretId,
		tmpSecretKey:    rspSt.Response.Credentials.TmpSecretKey,
		source:          r,
	}, nil
}
