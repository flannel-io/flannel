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

	When(`^I attempt to call the "(.+?)" API without the "(.+?)" parameter$`, func(s1 string, s2 string) {
		//		call(s1, nil, true)
		T.Skip() // pending
	})

	When(`^I attempt to call the "(.+?)" API with "(.+?)" parameter$`, func(s1 string, s2 string) {
		//		call(s1, nil, true)
		T.Skip() // pending
	})

	Then(`^the hostname should equal the "(.+?)" parameter$`, func(s1 string) {
		T.Skip() // pending
	})
}
