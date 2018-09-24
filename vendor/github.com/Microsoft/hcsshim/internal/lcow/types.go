package lcow

import "github.com/Microsoft/hcsshim/internal/schema2"

// Additional fields to hcsschema.ProcessParameters used by LCOW
type ProcessParameters struct {
	hcsschema.ProcessParameters

	CreateInUtilityVm bool        `json:",omitempty"`
	OCIProcess        interface{} `json:"OciProcess,omitempty"`
}
