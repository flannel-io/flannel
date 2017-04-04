package sms

import (
	"net/http"

	"github.com/denverdino/aliyungo/common"
)

type SendSmsArgs struct {
	SignName string
	TemplateCode  string
	RecNum        string
	ParamString   string
}

//please set the signature and template in the console of Aliyun before you call this API
func (this *Client) SendSms(args *SendSmsArgs) error {
	return this.InvokeByAnyMethod(http.MethodPost, SingleSendSms, args, &common.Response{})
}
