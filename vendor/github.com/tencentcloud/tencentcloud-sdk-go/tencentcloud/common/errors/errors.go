package errors

import (
	"fmt"
)

type TencentCloudSDKError struct {
	Code      string
	Message   string
	RequestId string
}

func (e *TencentCloudSDKError) Error() string {
	if e.RequestId == "" {
		return fmt.Sprintf("[TencentCloudSDKError] Code=%s, Message=%s", e.Code, e.Message)
	}
	return fmt.Sprintf("[TencentCloudSDKError] Code=%s, Message=%s, RequestId=%s", e.Code, e.Message, e.RequestId)
}

func NewTencentCloudSDKError(code, message, requestId string) error {
	return &TencentCloudSDKError{
		Code:      code,
		Message:   message,
		RequestId: requestId,
	}
}

func (e *TencentCloudSDKError) GetCode() string {
	return e.Code
}

func (e *TencentCloudSDKError) GetMessage() string {
	return e.Message
}

func (e *TencentCloudSDKError) GetRequestId() string {
	return e.RequestId
}
