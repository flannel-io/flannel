//Package redshift provides gucumber integration tests suppport.
package redshift

import (
	"github.com/aws/aws-sdk-go/internal/features/shared"
	"github.com/aws/aws-sdk-go/service/redshift"
	. "github.com/lsegal/gucumber"
)

var _ = shared.Imported

func init() {
	Before("@redshift", func() {
		World["client"] = redshift.New(nil)
	})
}
