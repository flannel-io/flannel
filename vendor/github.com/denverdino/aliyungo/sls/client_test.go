package sls

import (
	"github.com/denverdino/aliyungo/common"
	"testing"
)

const (
	AccessKeyId      = ""
	AccessKeySecret  = ""
	Region           = common.Hangzhou
	TestProjectName  = "test-project123"
	TestLogstoreName = "test-logstore"
)

func DefaultProject(t *testing.T) *Project {
	client := NewClient(Region, false, AccessKeyId, AccessKeySecret)
	err := client.CreateProject(TestProjectName, "description")
	if err != nil {
		if e, ok := err.(*Error); ok && e.Code != "ProjectAlreadyExist" {
			t.Fatalf("create project fail: %s", err.Error())
		}
	}
	p, err := client.Project(TestProjectName)
	if err != nil {
		t.Fatalf("get project fail: %s", err.Error())
	}
	//Create default logstore

	logstore := &Logstore{
		TTL:   2,
		Shard: 3,
		Name:  TestLogstoreName,
	}
	err = p.CreateLogstore(logstore)
	if err != nil {
		if e, ok := err.(*Error); ok && e.Code != "LogStoreAlreadyExist" {
			t.Fatalf("create logstore fail: %s", err.Error())
		}
	}

	return p
}

