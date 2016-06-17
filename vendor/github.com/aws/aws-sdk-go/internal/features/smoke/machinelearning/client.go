//Package machinelearning provides gucumber integration tests suppport.
package machinelearning

import (
	"github.com/aws/aws-sdk-go/internal/features/shared"
	"github.com/aws/aws-sdk-go/service/machinelearning"
	. "github.com/lsegal/gucumber"
)

var _ = shared.Imported

func init() {
	Before("@machinelearning", func() {
		World["client"] = machinelearning.New(nil)
	})
}
