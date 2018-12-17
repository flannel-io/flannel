package functional

import (
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcsoci"
	"github.com/sirupsen/logrus"
)

var pauseDurationOnCreateContainerFailure time.Duration

func init() {
	if len(os.Getenv("HCSSHIM_FUNCTIONAL_TESTS_DEBUG")) > 0 {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}

	// This allows for debugging a utility VM.
	s := os.Getenv("HCSSHIM_FUNCTIONAL_TESTS_PAUSE_ON_CREATECONTAINER_FAIL_IN_MINUTES")
	if s != "" {
		if t, err := strconv.Atoi(s); err == nil {
			pauseDurationOnCreateContainerFailure = time.Duration(t) * time.Minute
		}
	}

	// Try to stop any pre-existing compute processes
	cmd := exec.Command("powershell", `get-computeprocess | stop-computeprocess -force`)
	cmd.Run()

}

func CreateContainerTestWrapper(options *hcsoci.CreateOptions) (*hcs.System, *hcsoci.Resources, error) {
	if pauseDurationOnCreateContainerFailure != 0 {
		options.DoNotReleaseResourcesOnFailure = true
	}
	s, r, err := hcsoci.CreateContainer(options)
	if err != nil {
		logrus.Warnf("Test is pausing for %s for debugging CreateContainer failure", pauseDurationOnCreateContainerFailure)
		time.Sleep(pauseDurationOnCreateContainerFailure)
		hcsoci.ReleaseResources(r, options.HostingSystem, true)
	}
	return s, r, err
}
