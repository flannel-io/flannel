package common

import (
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

func NewCommonClient(cred CredentialIface, region string, clientProfile *profile.ClientProfile) (c *Client) {
	return new(Client).Init(region).WithCredential(cred).WithProfile(clientProfile)
}

// SendOctetStream Invoke API with application/octet-stream content-type.
//
// Note:
//  1. only specific API can be invoked in such manner.
//  2. only TC3-HMAC-SHA256 signature method can be specified.
//  3. only POST request method can be specified
//  4. the request Must be a CommonRequest and called SetOctetStreamParameters
//
func (c *Client) SendOctetStream(request tchttp.Request, response tchttp.Response) (err error) {
	if c.profile.SignMethod != "TC3-HMAC-SHA256" {
		return tcerr.NewTencentCloudSDKError("ClientError", "Invalid signature method.", "")
	}
	if c.profile.HttpProfile.ReqMethod != "POST" {
		return tcerr.NewTencentCloudSDKError("ClientError", "Invalid request method.", "")
	}
	//cr, ok := request.(*tchttp.CommonRequest)
	//if !ok {
	//	return tcerr.NewTencentCloudSDKError("ClientError", "Invalid request, must be *CommonRequest!", "")
	//}
	//if !cr.IsOctetStream() {
	//	return tcerr.NewTencentCloudSDKError("ClientError", "Invalid request, does not meet the conditions for sending OctetStream", "")
	//}
	return c.Send(request, response)
}
