// +build windows

package schemaversion

import (
	"encoding/json"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/sirupsen/logrus"
)

// SchemaV10 makes it easy for callers to get a v1.0 schema version object
func SchemaV10() *hcsschema.Version {
	return &hcsschema.Version{Major: 1, Minor: 0}
}

// SchemaV21 makes it easy for callers to get a v2.1 schema version object
func SchemaV21() *hcsschema.Version {
	return &hcsschema.Version{Major: 2, Minor: 1}
}

// isSupported determines if a given schema version is supported
func IsSupported(sv *hcsschema.Version) error {
	if IsV10(sv) {
		return nil
	}
	if IsV21(sv) {
		if osversion.Get().Build < osversion.RS5 {
			return fmt.Errorf("unsupported on this Windows build")
		}
		return nil
	}
	return fmt.Errorf("unknown schema version %s", String(sv))
}

// IsV10 determines if a given schema version object is 1.0. This was the only thing
// supported in RS1..3. It lives on in RS5, but will be deprecated in a future release.
func IsV10(sv *hcsschema.Version) bool {
	if sv.Major == 1 && sv.Minor == 0 {
		return true
	}
	return false
}

// IsV21 determines if a given schema version object is 2.0. This was introduced in
// RS4, but not fully implemented. Recommended for applications using HCS in RS5
// onwards.
func IsV21(sv *hcsschema.Version) bool {
	if sv.Major == 2 && sv.Minor == 1 {
		return true
	}
	return false
}

// String returns a JSON encoding of a schema version object
func String(sv *hcsschema.Version) string {
	b, err := json.Marshal(sv)
	if err != nil {
		return ""
	}
	return string(b[:])
}

// DetermineSchemaVersion works out what schema version to use based on build and
// requested option.
func DetermineSchemaVersion(requestedSV *hcsschema.Version) *hcsschema.Version {
	sv := SchemaV10()
	if osversion.Get().Build >= osversion.RS5 {
		sv = SchemaV21()
	}
	if requestedSV != nil {
		if err := IsSupported(requestedSV); err == nil {
			sv = requestedSV
		} else {
			logrus.Warnf("Ignoring unsupported requested schema version %+v", requestedSV)
		}
	}
	return sv
}
