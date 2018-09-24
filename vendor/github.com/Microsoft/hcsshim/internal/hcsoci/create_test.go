// +build windows,functional

//
// These unit tests must run on a system setup to run both Argons and Xenons,
// have docker installed, and have the nanoserver (WCOW) and alpine (LCOW)
// base images installed. The nanoserver image MUST match the build of the
// host.
//
// We rely on docker as the tools to extract a container image aren't
// open source. We use it to find the location of the base image on disk.
//

package hcsoci

//import (
//	"bytes"
//	"encoding/json"
//	"io/ioutil"
//	"os"
//	"os/exec"
//	"path/filepath"
//	"strings"
//	"testing"

//	"github.com/Microsoft/hcsshim/internal/schemaversion"
//	_ "github.com/Microsoft/hcsshim/test/assets"
//	specs "github.com/opencontainers/runtime-spec/specs-go"
//	"github.com/sirupsen/logrus"
//)

//func startUVM(t *testing.T, uvm *UtilityVM) {
//	if err := uvm.Start(); err != nil {
//		t.Fatalf("UVM %s Failed start: %s", uvm.Id, err)
//	}
//}

//// Helper to shoot a utility VM
//func terminateUtilityVM(t *testing.T, uvm *UtilityVM) {
//	if err := uvm.Terminate(); err != nil {
//		t.Fatalf("Failed terminate utility VM %s", err)
//	}
//}

//// TODO: Test UVMResourcesFromContainerSpec
//func TestUVMSizing(t *testing.T) {
//	t.Skip("for now - not implemented at all")
//}

//// TestID validates that the requested ID is retrieved
//func TestID(t *testing.T) {
//	t.Skip("fornow")
//	tempDir := createWCOWTempDirWithSandbox(t)
//	defer os.RemoveAll(tempDir)

//	layers := append(layersNanoserver, tempDir)
//	mountPath, err := mountContainerLayers(layers, nil)
//	if err != nil {
//		t.Fatalf("failed to mount container storage: %s", err)
//	}
//	defer unmountContainerLayers(layers, nil, unmountOperationAll)

//	c, err := CreateContainer(&CreateOptions{
//		Id:            "gruntbuggly",
//		SchemaVersion: schemaversion.SchemaV21(),
//		Spec: &specs.Spec{
//			Windows: &specs.Windows{LayerFolders: layers},
//			Root:    &specs.Root{Path: mountPath.(string)},
//		},
//	})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	if c.ID() != "gruntbuggly" {
//		t.Fatalf("id not set correctly: %s", c.ID())
//	}

//	c.Terminate()
//}
