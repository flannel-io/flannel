// Code generated by protoc-gen-go.
// source: google.golang.org/appengine/internal/urlfetch/urlfetch_service.proto
// DO NOT EDIT!

/*
Package urlfetch is a generated protocol buffer package.

It is generated from these files:
	google.golang.org/appengine/internal/urlfetch/urlfetch_service.proto

It has these top-level messages:
	URLFetchServiceError
	URLFetchRequest
	URLFetchResponse
*/
package urlfetch

import proto "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type URLFetchServiceError_ErrorCode int32

const (
	URLFetchServiceError_OK                       URLFetchServiceError_ErrorCode = 0
	URLFetchServiceError_INVALID_URL              URLFetchServiceError_ErrorCode = 1
	URLFetchServiceError_FETCH_ERROR              URLFetchServiceError_ErrorCode = 2
	URLFetchServiceError_UNSPECIFIED_ERROR        URLFetchServiceError_ErrorCode = 3
	URLFetchServiceError_RESPONSE_TOO_LARGE       URLFetchServiceError_ErrorCode = 4
	URLFetchServiceError_DEADLINE_EXCEEDED        URLFetchServiceError_ErrorCode = 5
	URLFetchServiceError_SSL_CERTIFICATE_ERROR    URLFetchServiceError_ErrorCode = 6
	URLFetchServiceError_DNS_ERROR                URLFetchServiceError_ErrorCode = 7
	URLFetchServiceError_CLOSED                   URLFetchServiceError_ErrorCode = 8
	URLFetchServiceError_INTERNAL_TRANSIENT_ERROR URLFetchServiceError_ErrorCode = 9
	URLFetchServiceError_TOO_MANY_REDIRECTS       URLFetchServiceError_ErrorCode = 10
	URLFetchServiceError_MALFORMED_REPLY          URLFetchServiceError_ErrorCode = 11
	URLFetchServiceError_CONNECTION_ERROR         URLFetchServiceError_ErrorCode = 12
)

var URLFetchServiceError_ErrorCode_name = map[int32]string{
	0:  "OK",
	1:  "INVALID_URL",
	2:  "FETCH_ERROR",
	3:  "UNSPECIFIED_ERROR",
	4:  "RESPONSE_TOO_LARGE",
	5:  "DEADLINE_EXCEEDED",
	6:  "SSL_CERTIFICATE_ERROR",
	7:  "DNS_ERROR",
	8:  "CLOSED",
	9:  "INTERNAL_TRANSIENT_ERROR",
	10: "TOO_MANY_REDIRECTS",
	11: "MALFORMED_REPLY",
	12: "CONNECTION_ERROR",
}
var URLFetchServiceError_ErrorCode_value = map[string]int32{
	"OK":                       0,
	"INVALID_URL":              1,
	"FETCH_ERROR":              2,
	"UNSPECIFIED_ERROR":        3,
	"RESPONSE_TOO_LARGE":       4,
	"DEADLINE_EXCEEDED":        5,
	"SSL_CERTIFICATE_ERROR":    6,
	"DNS_ERROR":                7,
	"CLOSED":                   8,
	"INTERNAL_TRANSIENT_ERROR": 9,
	"TOO_MANY_REDIRECTS":       10,
	"MALFORMED_REPLY":          11,
	"CONNECTION_ERROR":         12,
}

func (x URLFetchServiceError_ErrorCode) Enum() *URLFetchServiceError_ErrorCode {
	p := new(URLFetchServiceError_ErrorCode)
	*p = x
	return p
}
func (x URLFetchServiceError_ErrorCode) String() string {
	return proto.EnumName(URLFetchServiceError_ErrorCode_name, int32(x))
}
func (x *URLFetchServiceError_ErrorCode) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(URLFetchServiceError_ErrorCode_value, data, "URLFetchServiceError_ErrorCode")
	if err != nil {
		return err
	}
	*x = URLFetchServiceError_ErrorCode(value)
	return nil
}

type URLFetchRequest_RequestMethod int32

const (
	URLFetchRequest_GET    URLFetchRequest_RequestMethod = 1
	URLFetchRequest_POST   URLFetchRequest_RequestMethod = 2
	URLFetchRequest_HEAD   URLFetchRequest_RequestMethod = 3
	URLFetchRequest_PUT    URLFetchRequest_RequestMethod = 4
	URLFetchRequest_DELETE URLFetchRequest_RequestMethod = 5
	URLFetchRequest_PATCH  URLFetchRequest_RequestMethod = 6
)

var URLFetchRequest_RequestMethod_name = map[int32]string{
	1: "GET",
	2: "POST",
	3: "HEAD",
	4: "PUT",
	5: "DELETE",
	6: "PATCH",
}
var URLFetchRequest_RequestMethod_value = map[string]int32{
	"GET":    1,
	"POST":   2,
	"HEAD":   3,
	"PUT":    4,
	"DELETE": 5,
	"PATCH":  6,
}

func (x URLFetchRequest_RequestMethod) Enum() *URLFetchRequest_RequestMethod {
	p := new(URLFetchRequest_RequestMethod)
	*p = x
	return p
}
func (x URLFetchRequest_RequestMethod) String() string {
	return proto.EnumName(URLFetchRequest_RequestMethod_name, int32(x))
}
func (x *URLFetchRequest_RequestMethod) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(URLFetchRequest_RequestMethod_value, data, "URLFetchRequest_RequestMethod")
	if err != nil {
		return err
	}
	*x = URLFetchRequest_RequestMethod(value)
	return nil
}

type URLFetchServiceError struct {
	XXX_unrecognized []byte `json:"-"`
}

func (m *URLFetchServiceError) Reset()         { *m = URLFetchServiceError{} }
func (m *URLFetchServiceError) String() string { return proto.CompactTextString(m) }
func (*URLFetchServiceError) ProtoMessage()    {}

type URLFetchRequest struct {
	Method                        *URLFetchRequest_RequestMethod `protobuf:"varint,1,req,name=Method,enum=appengine.URLFetchRequest_RequestMethod" json:"Method,omitempty"`
	Url                           *string                        `protobuf:"bytes,2,req,name=Url" json:"Url,omitempty"`
	Header                        []*URLFetchRequest_Header      `protobuf:"group,3,rep,name=Header" json:"header,omitempty"`
	Payload                       []byte                         `protobuf:"bytes,6,opt,name=Payload" json:"Payload,omitempty"`
	FollowRedirects               *bool                          `protobuf:"varint,7,opt,name=FollowRedirects,def=1" json:"FollowRedirects,omitempty"`
	Deadline                      *float64                       `protobuf:"fixed64,8,opt,name=Deadline" json:"Deadline,omitempty"`
	MustValidateServerCertificate *bool                          `protobuf:"varint,9,opt,name=MustValidateServerCertificate,def=1" json:"MustValidateServerCertificate,omitempty"`
	XXX_unrecognized              []byte                         `json:"-"`
}

func (m *URLFetchRequest) Reset()         { *m = URLFetchRequest{} }
func (m *URLFetchRequest) String() string { return proto.CompactTextString(m) }
func (*URLFetchRequest) ProtoMessage()    {}

const Default_URLFetchRequest_FollowRedirects bool = true
const Default_URLFetchRequest_MustValidateServerCertificate bool = true

func (m *URLFetchRequest) GetMethod() URLFetchRequest_RequestMethod {
	if m != nil && m.Method != nil {
		return *m.Method
	}
	return URLFetchRequest_GET
}

func (m *URLFetchRequest) GetUrl() string {
	if m != nil && m.Url != nil {
		return *m.Url
	}
	return ""
}

func (m *URLFetchRequest) GetHeader() []*URLFetchRequest_Header {
	if m != nil {
		return m.Header
	}
	return nil
}

func (m *URLFetchRequest) GetPayload() []byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (m *URLFetchRequest) GetFollowRedirects() bool {
	if m != nil && m.FollowRedirects != nil {
		return *m.FollowRedirects
	}
	return Default_URLFetchRequest_FollowRedirects
}

func (m *URLFetchRequest) GetDeadline() float64 {
	if m != nil && m.Deadline != nil {
		return *m.Deadline
	}
	return 0
}

func (m *URLFetchRequest) GetMustValidateServerCertificate() bool {
	if m != nil && m.MustValidateServerCertificate != nil {
		return *m.MustValidateServerCertificate
	}
	return Default_URLFetchRequest_MustValidateServerCertificate
}

type URLFetchRequest_Header struct {
	Key              *string `protobuf:"bytes,4,req,name=Key" json:"Key,omitempty"`
	Value            *string `protobuf:"bytes,5,req,name=Value" json:"Value,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *URLFetchRequest_Header) Reset()         { *m = URLFetchRequest_Header{} }
func (m *URLFetchRequest_Header) String() string { return proto.CompactTextString(m) }
func (*URLFetchRequest_Header) ProtoMessage()    {}

func (m *URLFetchRequest_Header) GetKey() string {
	if m != nil && m.Key != nil {
		return *m.Key
	}
	return ""
}

func (m *URLFetchRequest_Header) GetValue() string {
	if m != nil && m.Value != nil {
		return *m.Value
	}
	return ""
}

type URLFetchResponse struct {
	Content               []byte                     `protobuf:"bytes,1,opt,name=Content" json:"Content,omitempty"`
	StatusCode            *int32                     `protobuf:"varint,2,req,name=StatusCode" json:"StatusCode,omitempty"`
	Header                []*URLFetchResponse_Header `protobuf:"group,3,rep,name=Header" json:"header,omitempty"`
	ContentWasTruncated   *bool                      `protobuf:"varint,6,opt,name=ContentWasTruncated,def=0" json:"ContentWasTruncated,omitempty"`
	ExternalBytesSent     *int64                     `protobuf:"varint,7,opt,name=ExternalBytesSent" json:"ExternalBytesSent,omitempty"`
	ExternalBytesReceived *int64                     `protobuf:"varint,8,opt,name=ExternalBytesReceived" json:"ExternalBytesReceived,omitempty"`
	FinalUrl              *string                    `protobuf:"bytes,9,opt,name=FinalUrl" json:"FinalUrl,omitempty"`
	ApiCpuMilliseconds    *int64                     `protobuf:"varint,10,opt,name=ApiCpuMilliseconds,def=0" json:"ApiCpuMilliseconds,omitempty"`
	ApiBytesSent          *int64                     `protobuf:"varint,11,opt,name=ApiBytesSent,def=0" json:"ApiBytesSent,omitempty"`
	ApiBytesReceived      *int64                     `protobuf:"varint,12,opt,name=ApiBytesReceived,def=0" json:"ApiBytesReceived,omitempty"`
	XXX_unrecognized      []byte                     `json:"-"`
}

func (m *URLFetchResponse) Reset()         { *m = URLFetchResponse{} }
func (m *URLFetchResponse) String() string { return proto.CompactTextString(m) }
func (*URLFetchResponse) ProtoMessage()    {}

const Default_URLFetchResponse_ContentWasTruncated bool = false
const Default_URLFetchResponse_ApiCpuMilliseconds int64 = 0
const Default_URLFetchResponse_ApiBytesSent int64 = 0
const Default_URLFetchResponse_ApiBytesReceived int64 = 0

func (m *URLFetchResponse) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

func (m *URLFetchResponse) GetStatusCode() int32 {
	if m != nil && m.StatusCode != nil {
		return *m.StatusCode
	}
	return 0
}

func (m *URLFetchResponse) GetHeader() []*URLFetchResponse_Header {
	if m != nil {
		return m.Header
	}
	return nil
}

func (m *URLFetchResponse) GetContentWasTruncated() bool {
	if m != nil && m.ContentWasTruncated != nil {
		return *m.ContentWasTruncated
	}
	return Default_URLFetchResponse_ContentWasTruncated
}

func (m *URLFetchResponse) GetExternalBytesSent() int64 {
	if m != nil && m.ExternalBytesSent != nil {
		return *m.ExternalBytesSent
	}
	return 0
}

func (m *URLFetchResponse) GetExternalBytesReceived() int64 {
	if m != nil && m.ExternalBytesReceived != nil {
		return *m.ExternalBytesReceived
	}
	return 0
}

func (m *URLFetchResponse) GetFinalUrl() string {
	if m != nil && m.FinalUrl != nil {
		return *m.FinalUrl
	}
	return ""
}

func (m *URLFetchResponse) GetApiCpuMilliseconds() int64 {
	if m != nil && m.ApiCpuMilliseconds != nil {
		return *m.ApiCpuMilliseconds
	}
	return Default_URLFetchResponse_ApiCpuMilliseconds
}

func (m *URLFetchResponse) GetApiBytesSent() int64 {
	if m != nil && m.ApiBytesSent != nil {
		return *m.ApiBytesSent
	}
	return Default_URLFetchResponse_ApiBytesSent
}

func (m *URLFetchResponse) GetApiBytesReceived() int64 {
	if m != nil && m.ApiBytesReceived != nil {
		return *m.ApiBytesReceived
	}
	return Default_URLFetchResponse_ApiBytesReceived
}

type URLFetchResponse_Header struct {
	Key              *string `protobuf:"bytes,4,req,name=Key" json:"Key,omitempty"`
	Value            *string `protobuf:"bytes,5,req,name=Value" json:"Value,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *URLFetchResponse_Header) Reset()         { *m = URLFetchResponse_Header{} }
func (m *URLFetchResponse_Header) String() string { return proto.CompactTextString(m) }
func (*URLFetchResponse_Header) ProtoMessage()    {}

func (m *URLFetchResponse_Header) GetKey() string {
	if m != nil && m.Key != nil {
		return *m.Key
	}
	return ""
}

func (m *URLFetchResponse_Header) GetValue() string {
	if m != nil && m.Value != nil {
		return *m.Value
	}
	return ""
}

func init() {
}
