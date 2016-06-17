//Package codedeploy provides gucumber integration tests suppport.
package codedeploy

import (
	"github.com/aws/aws-sdk-go/internal/features/shared"
	"github.com/aws/aws-sdk-go/service/codedeploy"
	. "github.com/lsegal/gucumber"
)

var _ = shared.Imported

func init() {
	Before("@codedeploy", func() {
		World["client"] = codedeploy.New(nil)
	})
}
