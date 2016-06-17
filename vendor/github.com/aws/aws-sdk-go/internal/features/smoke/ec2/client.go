//Package ec2 provides gucumber integration tests suppport.
package ec2

import (
	"github.com/aws/aws-sdk-go/internal/features/shared"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/lsegal/gucumber"
)

var _ = shared.Imported

func init() {
	Before("@ec2", func() {
		World["client"] = ec2.New(nil)
	})
}
