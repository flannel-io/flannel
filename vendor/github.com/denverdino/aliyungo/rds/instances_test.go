package rds

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var (
	UT_ACCESSKEYID     = os.Getenv("AccessKeyId")
	UT_ACCESSKEYSECRET = os.Getenv("AccessKeySecret")
)

func TestClient_ModifySecurityIps(t *testing.T) {

	if UT_ACCESSKEYID == "" {
		t.SkipNow()
	}
	client := NewClient(UT_ACCESSKEYID, UT_ACCESSKEYSECRET)

	// TODO:
	args := &ModifySecurityIpsArgs{
		DBInstanceId: "xxxx",
		SecurityIps:  "x.x.x.x,x.x.x.x",
	}
	resp, err := client.ModifySecurityIps(args)
	assert.Nil(t, err)
	assert.NotNil(t, resp)

	t.Logf("the result is %++v ", resp)
}
