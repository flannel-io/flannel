package common

import (
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
)

type ProviderChain struct {
	Providers []Provider
}

// NewProviderChain returns a provider chain in your custom order
func NewProviderChain(providers []Provider) Provider {
	return &ProviderChain{
		Providers: providers,
	}
}

// DefaultProviderChain returns a default provider chain and try to get credentials in the following order:
//  1. Environment variable
//  2. Profile
//  3. CvmRole
// If you want to customize the search order, please use the function NewProviderChain
func DefaultProviderChain() Provider {
	return NewProviderChain([]Provider{DefaultEnvProvider(), DefaultProfileProvider(), DefaultCvmRoleProvider()})
}

func (c *ProviderChain) GetCredential() (CredentialIface, error) {
	for _, provider := range c.Providers {
		cred, err := provider.GetCredential()
		if err != nil {
			if err == envNotSet || err == fileDoseNotExist || err == noCvmRole {
				continue
			} else {
				return nil, err
			}
		}
		return cred, err
	}
	return nil, tcerr.NewTencentCloudSDKError(creErr, "no credential found in every providers", "")

}
