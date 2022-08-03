package common

import (
	"log"
	"time"
)

type RoleArnCredential struct {
	roleArn         string
	roleSessionName string
	durationSeconds int64
	expiredTime     int64
	token           string
	tmpSecretId     string
	tmpSecretKey    string
	source          *RoleArnProvider
}

func (c *RoleArnCredential) GetSecretId() string {
	if c.needRefresh() {
		c.refresh()
	}
	return c.tmpSecretId
}

func (c *RoleArnCredential) GetSecretKey() string {
	if c.needRefresh() {
		c.refresh()
	}
	return c.tmpSecretKey

}

func (c *RoleArnCredential) GetToken() string {
	if c.needRefresh() {
		c.refresh()
	}
	return c.token
}

func (c *RoleArnCredential) needRefresh() bool {
	if c.tmpSecretKey == "" || c.tmpSecretId == "" || c.token == "" || c.expiredTime <= time.Now().Unix() {
		return true
	}
	return false
}

func (c *RoleArnCredential) refresh() {
	newCre, err := c.source.GetCredential()
	if err != nil {
		log.Println(err)
	}
	*c = *newCre.(*RoleArnCredential)
}
