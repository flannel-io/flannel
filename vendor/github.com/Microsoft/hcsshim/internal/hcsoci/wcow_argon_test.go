// +build windows,functional

package hcsoci

//import (
//	"os"
//	"path/filepath"
//	"testing"

//	"github.com/Microsoft/hcsshim/internal/schemaversion"
//	specs "github.com/opencontainers/runtime-spec/specs-go"
//)

//// --------------------------------
////    W C O W    A R G O N   V 1
//// --------------------------------

//// A v1 Argon with a single base layer. It also validates hostname functionality is propagated.
//func TestV1Argon(t *testing.T) {
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
//		SchemaVersion: schemaversion.SchemaV10(),
//		Id:            "TestV1Argon",
//		Owner:         "unit-test",
//		Spec: &specs.Spec{
//			Hostname: "goofy",
//			Windows:  &specs.Windows{LayerFolders: layers},
//			Root:     &specs.Root{Path: mountPath.(string)},
//		},
//	})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	startContainer(t, c)
//	runCommand(t, c, "cmd /s /c echo Hello", `c:\`, "Hello")
//	runCommand(t, c, "cmd /s /c hostname", `c:\`, "goofy")
//	stopContainer(t, c)
//	c.Terminate()
//}

//// A v1 Argon with a single base layer which uses the auto-mount capability
//func TestV1ArgonAutoMount(t *testing.T) {
//	t.Skip("fornow")
//	tempDir := createWCOWTempDirWithSandbox(t)
//	defer os.RemoveAll(tempDir)

//	layers := append(layersBusybox, tempDir)
//	c, err := CreateContainer(&CreateOptions{
//		Id:            "TestV1ArgonAutoMount",
//		SchemaVersion: schemaversion.SchemaV10(),
//		Spec:          &specs.Spec{Windows: &specs.Windows{LayerFolders: layers}},
//	})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	defer unmountContainerLayers(layers, nil, unmountOperationAll)
//	startContainer(t, c)
//	runCommand(t, c, "cmd /s /c echo Hello", `c:\`, "Hello")
//	stopContainer(t, c)
//	c.Terminate()
//}

//// A v1 Argon with multiple layers which uses the auto-mount capability
//func TestV1ArgonMultipleBaseLayersAutoMount(t *testing.T) {
//	t.Skip("fornow")

//	// This is the important bit for this test. It's deleted here. We call the helper only to allocate a temporary directory
//	containerScratchDir := createTempDir(t)
//	os.RemoveAll(containerScratchDir)
//	defer os.RemoveAll(containerScratchDir) // As auto-created

//	layers := append(layersBusybox, containerScratchDir)
//	c, err := CreateContainer(&CreateOptions{
//		Id:            "TestV1ArgonMultipleBaseLayersAutoMount",
//		SchemaVersion: schemaversion.SchemaV10(),
//		Spec:          &specs.Spec{Windows: &specs.Windows{LayerFolders: layers}},
//	})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	defer unmountContainerLayers(layers, nil, unmountOperationAll)
//	startContainer(t, c)
//	runCommand(t, c, "cmd /s /c echo Hello", `c:\`, "Hello")
//	stopContainer(t, c)
//	c.Terminate()
//}

//// A v1 Argon with a single mapped directory.
//func TestV1ArgonSingleMappedDirectory(t *testing.T) {
//	t.Skip("fornow")
//	tempDir := createWCOWTempDirWithSandbox(t)
//	defer os.RemoveAll(tempDir)

//	layers := append(layersNanoserver, tempDir)

//	// Create a temp folder containing foo.txt which will be used for the bind-mount test.
//	source := createTempDir(t)
//	defer os.RemoveAll(source)
//	mount := specs.Mount{
//		Source:      source,
//		Destination: `c:\foo`,
//	}
//	f, err := os.OpenFile(filepath.Join(source, "foo.txt"), os.O_RDWR|os.O_CREATE, 0755)
//	f.Close()

//	c, err := CreateContainer(&CreateOptions{
//		SchemaVersion: schemaversion.SchemaV10(),
//		Spec: &specs.Spec{
//			Windows: &specs.Windows{LayerFolders: layers},
//			Mounts:  []specs.Mount{mount},
//		},
//	})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	defer unmountContainerLayers(layers, nil, unmountOperationAll)

//	startContainer(t, c)
//	runCommand(t, c, `cmd /s /c dir /b c:\foo`, `c:\`, "foo.txt")
//	stopContainer(t, c)
//	c.Terminate()
//}

//// --------------------------------
////    W C O W    A R G O N   V 2
//// --------------------------------

//// A v2 Argon with a single base layer. It also validates hostname functionality is propagated.
//// It also uses an auto-generated ID.
//func TestV2Argon(t *testing.T) {
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
//		SchemaVersion: schemaversion.SchemaV21(),
//		Spec: &specs.Spec{
//			Hostname: "mickey",
//			Windows:  &specs.Windows{LayerFolders: layers},
//			Root:     &specs.Root{Path: mountPath.(string)},
//		},
//	})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	startContainer(t, c)
//	runCommand(t, c, "cmd /s /c echo Hello", `c:\`, "Hello")
//	runCommand(t, c, "cmd /s /c hostname", `c:\`, "mickey")
//	stopContainer(t, c)
//	c.Terminate()
//}

//// A v2 Argon with multiple layers
//func TestV2ArgonMultipleBaseLayers(t *testing.T) {
//	t.Skip("fornow")
//	tempDir := createWCOWTempDirWithSandbox(t)
//	defer os.RemoveAll(tempDir)

//	layers := append(layersBusybox, tempDir)
//	mountPath, err := mountContainerLayers(layers, nil)
//	if err != nil {
//		t.Fatalf("failed to mount container storage: %s", err)
//	}
//	defer unmountContainerLayers(layers, nil, unmountOperationAll)

//	c, err := CreateContainer(&CreateOptions{
//		SchemaVersion: schemaversion.SchemaV21(),
//		Id:            "TestV2ArgonMultipleBaseLayers",
//		Spec: &specs.Spec{
//			Windows: &specs.Windows{LayerFolders: layers},
//			Root:    &specs.Root{Path: mountPath.(string)},
//		},
//	})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	startContainer(t, c)
//	runCommand(t, c, "cmd /s /c echo Hello", `c:\`, "Hello")
//	stopContainer(t, c)
//	c.Terminate()
//}

//// A v2 Argon with multiple layers which uses the auto-mount capability and auto-create
//func TestV2ArgonAutoMountMultipleBaseLayers(t *testing.T) {
//	t.Skip("fornow")

//	// This is the important bit for this test. It's deleted here. We call the helper only to allocate a temporary directory
//	containerScratchDir := createTempDir(t)
//	os.RemoveAll(containerScratchDir)
//	defer os.RemoveAll(containerScratchDir) // As auto-created

//	layers := append(layersBusybox, containerScratchDir)

//	c, err := CreateContainer(&CreateOptions{
//		SchemaVersion: schemaversion.SchemaV21(),
//		Id:            "TestV2ArgonAutoMountMultipleBaseLayers",
//		Spec:          &specs.Spec{Windows: &specs.Windows{LayerFolders: layers}},
//	})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	defer unmountContainerLayers(layers, nil, unmountOperationAll)
//	startContainer(t, c)
//	runCommand(t, c, "cmd /s /c echo Hello", `c:\`, "Hello")
//	stopContainer(t, c)
//	c.Terminate()
//}

//// A v2 Argon with a single mapped directory.
//func TestV2ArgonSingleMappedDirectory(t *testing.T) {
//	t.Skip("fornow")
//	tempDir := createWCOWTempDirWithSandbox(t)
//	defer os.RemoveAll(tempDir)

//	layers := append(layersNanoserver, tempDir)

//	// Create a temp folder containing foo.txt which will be used for the bind-mount test.
//	source := createTempDir(t)
//	defer os.RemoveAll(source)
//	mount := specs.Mount{
//		Source:      source,
//		Destination: `c:\foo`,
//	}
//	f, err := os.OpenFile(filepath.Join(source, "foo.txt"), os.O_RDWR|os.O_CREATE, 0755)
//	f.Close()

//	c, err := CreateContainer(&CreateOptions{
//		SchemaVersion: schemaversion.SchemaV21(),
//		Spec: &specs.Spec{
//			Windows: &specs.Windows{LayerFolders: layers},
//			Mounts:  []specs.Mount{mount},
//		},
//	})
//	if err != nil {
//		t.Fatalf("Failed create: %s", err)
//	}
//	defer unmountContainerLayers(layers, nil, unmountOperationAll)

//	startContainer(t, c)
//	runCommand(t, c, `cmd /s /c dir /b c:\foo`, `c:\`, "foo.txt")
//	stopContainer(t, c)
//	c.Terminate()
//}
