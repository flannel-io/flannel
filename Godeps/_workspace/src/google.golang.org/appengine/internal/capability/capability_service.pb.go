// Code generated by protoc-gen-go.
// source: google.golang.org/appengine/internal/capability/capability_service.proto
// DO NOT EDIT!

/*
Package capability is a generated protocol buffer package.

It is generated from these files:
	google.golang.org/appengine/internal/capability/capability_service.proto

It has these top-level messages:
	IsEnabledRequest
	IsEnabledResponse
*/
package capability

import proto "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type IsEnabledResponse_SummaryStatus int32

const (
	IsEnabledResponse_DEFAULT          IsEnabledResponse_SummaryStatus = 0
	IsEnabledResponse_ENABLED          IsEnabledResponse_SummaryStatus = 1
	IsEnabledResponse_SCHEDULED_FUTURE IsEnabledResponse_SummaryStatus = 2
	IsEnabledResponse_SCHEDULED_NOW    IsEnabledResponse_SummaryStatus = 3
	IsEnabledResponse_DISABLED         IsEnabledResponse_SummaryStatus = 4
	IsEnabledResponse_UNKNOWN          IsEnabledResponse_SummaryStatus = 5
)

var IsEnabledResponse_SummaryStatus_name = map[int32]string{
	0: "DEFAULT",
	1: "ENABLED",
	2: "SCHEDULED_FUTURE",
	3: "SCHEDULED_NOW",
	4: "DISABLED",
	5: "UNKNOWN",
}
var IsEnabledResponse_SummaryStatus_value = map[string]int32{
	"DEFAULT":          0,
	"ENABLED":          1,
	"SCHEDULED_FUTURE": 2,
	"SCHEDULED_NOW":    3,
	"DISABLED":         4,
	"UNKNOWN":          5,
}

func (x IsEnabledResponse_SummaryStatus) Enum() *IsEnabledResponse_SummaryStatus {
	p := new(IsEnabledResponse_SummaryStatus)
	*p = x
	return p
}
func (x IsEnabledResponse_SummaryStatus) String() string {
	return proto.EnumName(IsEnabledResponse_SummaryStatus_name, int32(x))
}
func (x *IsEnabledResponse_SummaryStatus) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(IsEnabledResponse_SummaryStatus_value, data, "IsEnabledResponse_SummaryStatus")
	if err != nil {
		return err
	}
	*x = IsEnabledResponse_SummaryStatus(value)
	return nil
}

type IsEnabledRequest struct {
	Package          *string  `protobuf:"bytes,1,req,name=package" json:"package,omitempty"`
	Capability       []string `protobuf:"bytes,2,rep,name=capability" json:"capability,omitempty"`
	Call             []string `protobuf:"bytes,3,rep,name=call" json:"call,omitempty"`
	XXX_unrecognized []byte   `json:"-"`
}

func (m *IsEnabledRequest) Reset()         { *m = IsEnabledRequest{} }
func (m *IsEnabledRequest) String() string { return proto.CompactTextString(m) }
func (*IsEnabledRequest) ProtoMessage()    {}

func (m *IsEnabledRequest) GetPackage() string {
	if m != nil && m.Package != nil {
		return *m.Package
	}
	return ""
}

func (m *IsEnabledRequest) GetCapability() []string {
	if m != nil {
		return m.Capability
	}
	return nil
}

func (m *IsEnabledRequest) GetCall() []string {
	if m != nil {
		return m.Call
	}
	return nil
}

type IsEnabledResponse struct {
	SummaryStatus      *IsEnabledResponse_SummaryStatus `protobuf:"varint,1,opt,name=summary_status,enum=appengine.IsEnabledResponse_SummaryStatus" json:"summary_status,omitempty"`
	TimeUntilScheduled *int64                           `protobuf:"varint,2,opt,name=time_until_scheduled" json:"time_until_scheduled,omitempty"`
	XXX_unrecognized   []byte                           `json:"-"`
}

func (m *IsEnabledResponse) Reset()         { *m = IsEnabledResponse{} }
func (m *IsEnabledResponse) String() string { return proto.CompactTextString(m) }
func (*IsEnabledResponse) ProtoMessage()    {}

func (m *IsEnabledResponse) GetSummaryStatus() IsEnabledResponse_SummaryStatus {
	if m != nil && m.SummaryStatus != nil {
		return *m.SummaryStatus
	}
	return IsEnabledResponse_DEFAULT
}

func (m *IsEnabledResponse) GetTimeUntilScheduled() int64 {
	if m != nil && m.TimeUntilScheduled != nil {
		return *m.TimeUntilScheduled
	}
	return 0
}
