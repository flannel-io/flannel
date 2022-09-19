package common

import (
	"encoding/json"
	"fmt"
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
)

const (
	octetStream = "application/octet-stream"
)

type actionParameters map[string]interface{}

type CommonRequest struct {
	*BaseRequest
	actionParameters
}

func NewCommonRequest(service, version, action string) (request *CommonRequest) {
	request = &CommonRequest{
		BaseRequest:      &BaseRequest{},
		actionParameters: actionParameters{},
	}
	request.Init().WithApiInfo(service, version, action)
	return
}

// SetActionParameters set common request's actionParameters to your data.
// note: your data Must be a json-formatted string or byte array or map[string]interface{}
// note: you could not call SetActionParameters and SetOctetStreamParameters at once
func (cr *CommonRequest) SetActionParameters(data interface{}) error {
	if data == nil {
		return nil
	}
	switch data.(type) {
	case []byte:
		if err := json.Unmarshal(data.([]byte), &cr.actionParameters); err != nil {
			msg := fmt.Sprintf("Fail to parse contenst %s to json,because: %s", data.([]byte), err)
			return tcerr.NewTencentCloudSDKError("ClientError.ParseJsonError", msg, "")
		}
	case string:
		if err := json.Unmarshal([]byte(data.(string)), &cr.actionParameters); err != nil {
			msg := fmt.Sprintf("Fail to parse contenst %s to json,because: %s", data.(string), err)
			return tcerr.NewTencentCloudSDKError("ClientError.ParseJsonError", msg, "")
		}
	case map[string]interface{}:
		cr.actionParameters = data.(map[string]interface{})
	default:
		msg := fmt.Sprintf("Invalid data type:%T, must be one of the following: []byte, string, map[string]interface{}", data)
		return tcerr.NewTencentCloudSDKError("ClientError.InvalidParameter", msg, "")
	}
	return nil
}

func (cr *CommonRequest) IsOctetStream() bool {
	v, ok := cr.GetHeader()["Content-Type"]
	if !ok || v != octetStream {
		return false
	}
	value, ok := cr.actionParameters["OctetStreamBody"]
	if !ok {
		return false
	}
	_, ok = value.([]byte)
	if !ok {
		return false
	}
	return true
}

func (cr *CommonRequest) SetHeader(header map[string]string) {
	if header == nil {
		return
	}
	if cr.BaseRequest == nil {
		cr.BaseRequest = &BaseRequest{}
	}
	cr.BaseRequest.SetHeader(header)
}

func (cr *CommonRequest) GetHeader() map[string]string {
	if cr.BaseRequest == nil {
		return nil
	}
	return cr.BaseRequest.GetHeader()
}

// SetOctetStreamParameters set request body to your data, and set head Content-Type to application/octet-stream
// note: you could not call SetActionParameters and SetOctetStreamParameters on the same request
func (cr *CommonRequest) SetOctetStreamParameters(header map[string]string, body []byte) {
	parameter := map[string]interface{}{}
	if header == nil {
		header = map[string]string{}
	}
	header["Content-Type"] = octetStream
	cr.SetHeader(header)
	parameter["OctetStreamBody"] = body
	cr.actionParameters = parameter
}

func (cr *CommonRequest) GetOctetStreamBody() []byte {
	if cr.IsOctetStream() {
		return cr.actionParameters["OctetStreamBody"].([]byte)
	} else {
		return nil
	}
}

func (cr *CommonRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(cr.actionParameters)
}
