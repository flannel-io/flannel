// +build !integration

package glacier_test

import (
	"bytes"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/internal/test/unit"
	"github.com/aws/aws-sdk-go/service/glacier"
	"github.com/stretchr/testify/assert"
)

var (
	_ = unit.Imported

	payloadBuf = func() *bytes.Reader {
		buf := make([]byte, 5767168) // 5.5MB buffer
		for i := range buf {
			buf[i] = '0' // Fill with zero characters
		}
		return bytes.NewReader(buf)
	}()

	svc = glacier.New(nil)
)

func TestCustomizations(t *testing.T) {
	req, _ := svc.UploadArchiveRequest(&glacier.UploadArchiveInput{
		VaultName: aws.String("vault"),
		Body:      payloadBuf,
	})
	err := req.Build()
	assert.NoError(t, err)

	// Sets API version
	assert.Equal(t, req.Service.APIVersion, req.HTTPRequest.Header.Get("x-amz-glacier-version"))

	// Sets Account ID
	v := awsutil.ValuesAtPath(req.Params, "AccountId")
	assert.Equal(t, "-", v[0])

	// Computes checksums
	linear := "68aff0c5a91aa0491752bfb96e3fef33eb74953804f6a2f7b708d5bcefa8ff6b"
	tree := "154e26c78fd74d0c2c9b3cc4644191619dc4f2cd539ae2a74d5fd07957a3ee6a"
	assert.Equal(t, linear, req.HTTPRequest.Header.Get("x-amz-content-sha256"))
	assert.Equal(t, tree, req.HTTPRequest.Header.Get("x-amz-sha256-tree-hash"))
}

func TestShortcircuitTreehash(t *testing.T) {
	req, _ := svc.UploadArchiveRequest(&glacier.UploadArchiveInput{
		VaultName: aws.String("vault"),
		Body:      payloadBuf,
		Checksum:  aws.String("000"),
	})
	err := req.Build()
	assert.NoError(t, err)

	assert.Equal(t, "000", req.HTTPRequest.Header.Get("x-amz-sha256-tree-hash"))
}

func TestFillAccountIDWithNilStruct(t *testing.T) {
	req, _ := svc.ListVaultsRequest(nil)
	err := req.Build()
	assert.NoError(t, err)

	empty := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Sets Account ID
	v := awsutil.ValuesAtPath(req.Params, "AccountId")
	assert.Equal(t, "-", v[0])

	// Does not set tree hash
	assert.Equal(t, empty, req.HTTPRequest.Header.Get("x-amz-content-sha256"))
	assert.Equal(t, "", req.HTTPRequest.Header.Get("x-amz-sha256-tree-hash"))
}
