package uvmfolder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// LocateUVMFolder searches a set of layer folders to determine the "uppermost"
// layer which has a utility VM image. The order of the layers is (for historical) reasons
// Read-only-layers followed by an optional read-write layer. The RO layers are in reverse
// order so that the upper-most RO layer is at the start, and the base OS layer is the
// end.
func LocateUVMFolder(layerFolders []string) (string, error) {
	var uvmFolder string
	index := 0
	for _, layerFolder := range layerFolders {
		_, err := os.Stat(filepath.Join(layerFolder, `UtilityVM`))
		if err == nil {
			uvmFolder = layerFolder
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		index++
	}
	if uvmFolder == "" {
		return "", fmt.Errorf("utility VM folder could not be found in layers")
	}
	logrus.Debugf("hcsshim::LocateUVMFolder At %d of %d: %s", index+1, len(layerFolders), uvmFolder)
	return uvmFolder, nil
}
