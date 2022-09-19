package common

import (
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"os"
)

type EnvProvider struct {
	secretIdENV  string
	secretKeyENV string
}

// DefaultEnvProvider return a default provider
// The default environment variable name are TENCENTCLOUD_SECRET_ID and TENCENTCLOUD_SECRET_KEY
func DefaultEnvProvider() *EnvProvider {
	return &EnvProvider{
		secretIdENV:  "TENCENTCLOUD_SECRET_ID",
		secretKeyENV: "TENCENTCLOUD_SECRET_KEY",
	}
}

// NewEnvProvider uses the name of the environment variable you specified to get the credentials
func NewEnvProvider(secretIdEnvName, secretKeyEnvName string) *EnvProvider {
	return &EnvProvider{
		secretIdENV:  secretIdEnvName,
		secretKeyENV: secretKeyEnvName,
	}
}

func (p *EnvProvider) GetCredential() (CredentialIface, error) {
	secretId, ok1 := os.LookupEnv(p.secretIdENV)
	secretKey, ok2 := os.LookupEnv(p.secretKeyENV)
	if !ok1 || !ok2 {
		return nil, envNotSet
	}
	if secretId == "" || secretKey == "" {
		return nil, tcerr.NewTencentCloudSDKError(creErr, "Environmental variable ("+p.secretIdENV+" or "+p.secretKeyENV+") is empty", "")
	}
	return NewCredential(secretId, secretKey), nil
}
