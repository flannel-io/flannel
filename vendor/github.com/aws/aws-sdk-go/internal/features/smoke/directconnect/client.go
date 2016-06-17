//Package directconnect provides gucumber integration tests suppport.
package directconnect

import (
	"github.com/aws/aws-sdk-go/internal/features/shared"
	"github.com/aws/aws-sdk-go/service/directconnect"
	. "github.com/lsegal/gucumber"
)

var _ = shared.Imported

func init() {
	Before("@directconnect", func() {
		World["client"] = directconnect.New(nil)
	})
}
