package testutilities

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var imageLayers map[string][]string

func init() {
	imageLayers = make(map[string][]string)
}

func LayerFolders(t *testing.T, imageName string) []string {
	if _, ok := imageLayers[imageName]; !ok {
		imageLayers[imageName] = getLayers(t, imageName)
	}
	return imageLayers[imageName]
}

func getLayers(t *testing.T, imageName string) []string {
	cmd := exec.Command("docker", "inspect", imageName, "-f", `"{{.GraphDriver.Data.dir}}"`)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Skipf("Failed to find layers for %q. Check docker images", imageName)
	}
	imagePath := strings.Replace(strings.TrimSpace(out.String()), `"`, ``, -1)
	layers := getLayerChain(t, imagePath)
	return append([]string{imagePath}, layers...)
}

func getLayerChain(t *testing.T, layerFolder string) []string {
	jPath := filepath.Join(layerFolder, "layerchain.json")
	content, err := ioutil.ReadFile(jPath)
	if os.IsNotExist(err) {
		t.Fatalf("layerchain not found")
	} else if err != nil {
		t.Fatalf("failed to read layerchain")
	}

	var layerChain []string
	err = json.Unmarshal(content, &layerChain)
	if err != nil {
		t.Fatalf("failed to unmarshal layerchain")
	}
	return layerChain
}
