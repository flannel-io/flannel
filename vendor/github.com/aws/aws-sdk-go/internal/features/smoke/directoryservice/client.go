//Package directoryservice provides gucumber integration tests suppport.
package directoryservice

import (
	"github.com/aws/aws-sdk-go/internal/features/shared"
	"github.com/aws/aws-sdk-go/service/directoryservice"
	. "github.com/lsegal/gucumber"
)

var _ = shared.Imported

func init() {
	Before("@directoryservice", func() {
		World["client"] = directoryservice.New(nil)
	})
}
