package common

import (
	"log"
	"time"
)

const ExpiredTimeout = 300

type CvmRoleCredential struct {
	roleName     string
	expiredTime  int64
	tmpSecretId  string
	tmpSecretKey string
	token        string
	source       *CvmRoleProvider
}

func (c *CvmRoleCredential) GetSecretId() string {
	if c.needRefresh() {
		c.refresh()
	}
	return c.tmpSecretId
}

func (c *CvmRoleCredential) GetToken() string {
	if c.needRefresh() {
		c.refresh()
	}
	return c.token
}

func (c *CvmRoleCredential) GetSecretKey() string {
	if c.needRefresh() {
		c.refresh()
	}
	return c.tmpSecretKey
}

func (c *CvmRoleCredential) needRefresh() bool {
	if c.tmpSecretId == "" || c.tmpSecretKey == "" || c.token == "" || c.expiredTime-ExpiredTimeout <= time.Now().Unix() {
		return true
	}
	return false
}

func (c *CvmRoleCredential) refresh() {
	newCre, err := c.source.GetCredential()
	if err != nil {
		log.Println(err)
		return
	}
	*c = *newCre.(*CvmRoleCredential)
}
