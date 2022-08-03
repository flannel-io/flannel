package common

import tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"

var (
	envNotSet        = tcerr.NewTencentCloudSDKError(creErr, "could not find environmental variable", "")
	fileDoseNotExist = tcerr.NewTencentCloudSDKError(creErr, "could not find config file", "")
	noCvmRole        = tcerr.NewTencentCloudSDKError(creErr, "get cvm role name failed, Please confirm whether the role is bound", "")
)

// Provider provide credential to build client.
//
// Now there are four kinds provider:
//  EnvProvider : get credential from your Variable environment
//  ProfileProvider : get credential from your profile
//	CvmRoleProvider : get credential from your cvm role
//  RoleArnProvider : get credential from your role arn
type Provider interface {
	// GetCredential get the credential interface
	GetCredential() (CredentialIface, error)
}
