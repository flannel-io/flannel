// xxxxbuild functional wcow wcowv2 wcowv2xenon

package functional

//import (
//	"os"
//	"testing"

//	"github.com/Microsoft/hcsshim/test/functional/utilities"
//	"github.com/Microsoft/hcsshim/internal/guid"
//	"github.com/Microsoft/hcsshim/internal/hcsoci"
//	"github.com/Microsoft/hcsshim/osversion"
//	"github.com/Microsoft/hcsshim/internal/schemaversion"
//	"github.com/Microsoft/hcsshim/internal/uvm"
//	"github.com/Microsoft/hcsshim/internal/uvmfolder"
//	"github.com/Microsoft/hcsshim/internal/wclayer"
//	"github.com/Microsoft/hcsshim/internal/wcow"
//	specs "github.com/opencontainers/runtime-spec/specs-go"
//)

// TODO. This might be worth porting.
//// Lots of v2 WCOW containers in the same UVM, each with a single base layer. Containers aren't
//// actually started, but it stresses the SCSI controller hot-add logic.
//func TestV2XenonWCOWCreateLots(t *testing.T) {
//	t.Skip("Skipping for now")
//	uvm, uvmScratchDir := createv2WCOWUVM(t, layersNanoserver, "TestV2XenonWCOWCreateLots", nil)
//	defer os.RemoveAll(uvmScratchDir)
//	defer uvm.Close()

//	// 63 as 0:0 is already taken as the UVMs scratch. So that leaves us with 64-1 left for container scratches on SCSI
//	for i := 0; i < 63; i++ {
//		containerScratchDir := createWCOWTempDirWithSandbox(t)
//		defer os.RemoveAll(containerScratchDir)
//		layerFolders := append(layersNanoserver, containerScratchDir)
//		hostedContainer, err := CreateContainer(&CreateOptions{
//			Id:            fmt.Sprintf("container%d", i),
//			HostingSystem: uvm,
//			SchemaVersion: schemaversion.SchemaV21(),
//			Spec:          &specs.Spec{Windows: &specs.Windows{LayerFolders: layerFolders}},
//		})
//		if err != nil {
//			t.Fatalf("CreateContainer failed: %s", err)
//		}
//		defer hostedContainer.Terminate()
//		defer unmountContainerLayers(layerFolders, uvm, unmountOperationAll)
//	}

//	// TODO: Should check the internal structures here for VSMB and SCSI

//	// TODO: Push it over 63 now and will get a failure.
//}
