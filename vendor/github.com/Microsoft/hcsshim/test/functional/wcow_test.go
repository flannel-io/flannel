// +build functional wcow

package functional

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcsoci"
	"github.com/Microsoft/hcsshim/internal/schema1"
	"github.com/Microsoft/hcsshim/internal/schemaversion"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/internal/uvmfolder"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/Microsoft/hcsshim/internal/wcow"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/test/functional/utilities"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Has testing for Windows containers using both the older hcsshim methods,
// and the newer hcsoci methods. Does the same thing in six different ways:
//    - hcsshim/argon
//    - hcsshim/xenon
//    - hcsoci/argon v1
//    - hcsoci/xenon v1
//    - hcsoci/argon v2
//    - hcsoci/xenon v2
//
// Sample v1 HCS document for Xenon (no networking):
//
//{
//    "SystemType": "Container",
//    "Name": "48347b95d0ad4f37de6d1979b986fb65912f973ad4549fbe716e848679dfa25c",
//    "IgnoreFlushesDuringBoot": true,
//    "LayerFolderPath": "C:\\layers\\48347b95d0ad4f37de6d1979b986fb65912f973ad4549fbe716e848679dfa25c",
//    "Layers": [
//        {
//            "ID": "7095521e-b79e-50fc-bafb-958d85400362",
//            "Path": "C:\\layers\\f9b22d909166dd54b870eb699d54f4cf36d99f035ffd7701aff1267230aefd1e"
//        }
//    ],
//    "HvPartition": true,
//    "HvRuntime": {
//        "ImagePath": "C:\\layers\\f9b22d909166dd54b870eb699d54f4cf36d99f035ffd7701aff1267230aefd1e\\UtilityVM"
//    }
//}
//
// Sample v1 HCS document for Argon (no networking):
//
//{
//    "SystemType": "Container",
//    "Name": "0a8bb9ec8366aa48a8e5f810274701d8d4452989bf268fc338570bfdecddf8df",
//    "VolumePath": "\\\\?\\Volume{85da95c9-dda9-42e0-a066-40bd120c6f3c}",
//    "IgnoreFlushesDuringBoot": true,
//    "LayerFolderPath": "C:\\layers\\0a8bb9ec8366aa48a8e5f810274701d8d4452989bf268fc338570bfdecddf8df",
//    "Layers": [
//        {
//            "ID": "7095521e-b79e-50fc-bafb-958d85400362",
//            "Path": "C:\\layers\\f9b22d909166dd54b870eb699d54f4cf36d99f035ffd7701aff1267230aefd1e"
//        }
//    ],
//    "HvPartition": false
//}
//
// Sample v2 HCS document for Argon (no networking):
//
//{
//    "Owner": "sample",
//    "SchemaVersion": {
//        "Major": 2,
//        "Minor": 0
//    },
//    "Container": {
//        "Storage": {
//            "Layers": [
//                {
//                    "Id": "6ba9cac1-7086-5ee9-a197-c465d3f50ad7",
//                    "Path": "C:\\layers\\f30368666ce4457e86fe12867506e508071d89e7eae615fc389c64f2e37ce54e"
//                },
//                {
//                    "Id": "300b3ac0-b603-5367-9494-afec045dd369",
//                    "Path": "C:\\layers\\7a6ad2b849a9d29e6648d9950c1975b0f614a63b5fe2803009ce131745abcc62"
//                },
//                {
//                    "Id": "fa3057d9-0d4b-54c0-b2d5-34b7afc78f91",
//                    "Path": "C:\\layers\\5d1332fe416f7932c344ce9c536402a6fc6d0bfcdf7a74f67cc67b8cfc66ab41"
//                },
//                {
//                    "Id": "23284a2c-cdda-582a-a175-a196211b03cb",
//                    "Path": "C:\\layers\\b95977ad18f8fa04e517daa2e814f73d69bfff55c3ea68d56f2b0b8ae23a235d"
//                },
//                {
//                    "Id": "e0233918-d93f-5b08-839e-0cbeda79b68b",
//                    "Path": "C:\\layers\\b2a444ff0e984ef282d6a8e24fa0108e76b6807d943e111a0e878c1c53ed8246"
//                },
//                {
//                    "Id": "02740e08-d1d3-5715-9c08-c255eab4ca01",
//                    "Path": "C:\\layers\\de6b1a908240cca2aef34f49994e7d4e25a8e157a2cef3b6d6cf2d8e6400bfc2"
//                }
//            ],
//            "Path": "\\\\?\\Volume{baac0fd5-16b7-405b-9621-112aa8e3d973}\\"
//        }
//    },
//    "ShouldTerminateOnLastHandleClosed": true
//}
//
//
// Sample v2 HCS document for Xenon (no networking)
//
//{
//    "Owner": "functional.test.exe",
//    "SchemaVersion": {
//        "Major": 2,
//        "Minor": 0
//    },
//    "HostingSystemId": "xenonOci2UVM",
//    "HostedSystem": {
//        "SchemaVersion": {
//            "Major": 2,
//            "Minor": 0
//        },
//        "Container": {
//            "Storage": {
//                "Layers": [
//                    {
//                        "Id": "6ba9cac1-7086-5ee9-a197-c465d3f50ad7",
//                        "Path": "\\\\?\\VMSMB\\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\\s1"
//                    },
//                    {
//                        "Id": "300b3ac0-b603-5367-9494-afec045dd369",
//                        "Path": "\\\\?\\VMSMB\\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\\s2"
//                    },
//                    {
//                        "Id": "fa3057d9-0d4b-54c0-b2d5-34b7afc78f91",
//                        "Path": "\\\\?\\VMSMB\\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\\s3"
//                    },
//                    {
//                        "Id": "23284a2c-cdda-582a-a175-a196211b03cb",
//                        "Path": "\\\\?\\VMSMB\\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\\4"
//                    },
//                    {
//                        "Id": "e0233918-d93f-5b08-839e-0cbeda79b68b",
//                        "Path": "\\\\?\\VMSMB\\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\\s5"
//                    },
//                    {
//                        "Id": "02740e08-d1d3-5715-9c08-c255eab4ca01",
//                        "Path": "\\\\?\\VMSMB\\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\\s6"
//                    }
//                ],
//                "Path": "C:\\c\\1\\scratch"
//            },
//            "MappedDirectories": [
//                {
//                    "HostPath": "\\\\?\\VMSMB\\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\\s7",
//                    "ContainerPath": "c:\\mappedro",
//                    "ReadOnly": true
//                },
//                {
//                    "HostPath": "\\\\?\\VMSMB\\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\\s8",
//                    "ContainerPath": "c:\\mappedrw"
//                }
//            ]
//        }
//    },
//    "ShouldTerminateOnLastHandleClosed": true
//}

// Helper to start a container.
// Ones created through hcsoci methods will be of type *hcs.System.
// Ones created through hcsshim methods will be of type hcsshim.Container
func startContainer(t *testing.T, c interface{}) {
	var err error
	switch c.(type) {
	case *hcs.System:
		err = c.(*hcs.System).Start()
	case hcsshim.Container:
		err = c.(hcsshim.Container).Start()
	default:
		t.Fatal("unknown type")
	}
	if err != nil {
		t.Fatalf("Failed start: %s", err)
	}
}

// Helper to stop a container.
// Ones created through hcsoci methods will be of type *hcs.System.
// Ones created through hcsshim methods will be of type hcsshim.Container
func stopContainer(t *testing.T, c interface{}) {

	switch c.(type) {
	case *hcs.System:
		if err := c.(*hcs.System).Shutdown(); err != nil {
			if hcsshim.IsPending(err) {
				if err := c.(*hcs.System).Wait(); err != nil {
					t.Fatalf("Failed Wait shutdown: %s", err)
				}
			} else {
				t.Fatalf("Failed shutdown: %s", err)
			}
		}
		c.(*hcs.System).Terminate()

	case hcsshim.Container:
		if err := c.(hcsshim.Container).Shutdown(); err != nil {
			if hcsshim.IsPending(err) {
				if err := c.(hcsshim.Container).Wait(); err != nil {
					t.Fatalf("Failed Wait shutdown: %s", err)
				}
			} else {
				t.Fatalf("Failed shutdown: %s", err)
			}
		}
		c.(hcsshim.Container).Terminate()
	default:
		t.Fatalf("unknown type")
	}
}

// Helper to launch a process in a container created through the hcsshim methods.
// At the point of calling, the container must have been successfully created.
func runShimCommand(t *testing.T,
	c hcsshim.Container,
	command string,
	workdir string,
	expectedExitCode int,
	expectedOutput string) {

	if c == nil {
		t.Fatalf("requested container to start is nil!")
	}
	p, err := c.CreateProcess(&hcsshim.ProcessConfig{
		CommandLine:      command,
		WorkingDirectory: workdir,
		CreateStdInPipe:  true,
		CreateStdOutPipe: true,
		CreateStdErrPipe: true,
	})
	if err != nil {
		t.Fatalf("Failed Create Process: %s", err)

	}
	defer p.Close()
	if err := p.Wait(); err != nil {
		t.Fatalf("Failed Wait Process: %s", err)
	}
	exitCode, err := p.ExitCode()
	if err != nil {
		t.Fatalf("Failed to obtain process exit code: %s", err)
	}
	if exitCode != expectedExitCode {
		t.Fatalf("Exit code from %s wasn't %d (%d)", command, expectedExitCode, exitCode)
	}
	_, o, _, err := p.Stdio()
	if err != nil {
		t.Fatalf("Failed to get Stdio handles for process: %s", err)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(o)
	out := strings.TrimSpace(buf.String())
	if expectedOutput != "" {
		if out != expectedOutput {
			t.Fatalf("Failed to get %q from process: %q", expectedOutput, out)
		}
	}
}

func runShimCommands(t *testing.T, c hcsshim.Container) {
	runShimCommand(t, c, `echo Hello`, `c:\`, 0, "Hello")

	// Check that read-only doesn't allow deletion or creation
	runShimCommand(t, c, `ls c:\mappedro\readonly`, `c:\`, 0, `c:\mappedro\readonly`)
	runShimCommand(t, c, `rm c:\mappedro\readonly`, `c:\`, 1, "")
	runShimCommand(t, c, `cp readonly fail`, `c:\mappedro`, 1, "")
	runShimCommand(t, c, `ls`, `c:\mappedro`, 0, `readonly`)

	// Check that read-write allows new file creation and removal
	runShimCommand(t, c, `ls`, `c:\mappedrw`, 0, `readwrite`)
	runShimCommand(t, c, `cp readwrite succeeds`, `c:\mappedrw`, 0, "")
	runShimCommand(t, c, `ls`, `c:\mappedrw`, 0, "readwrite\nsucceeds")
	runShimCommand(t, c, `rm succeeds`, `c:\mappedrw`, 0, "")
	runShimCommand(t, c, `ls`, `c:\mappedrw`, 0, `readwrite`)
}

func runHcsCommands(t *testing.T, c *hcs.System) {
	runHcsCommand(t, c, `echo Hello`, `c:\`, 0, "Hello")

	// Check that read-only doesn't allow deletion or creation
	runHcsCommand(t, c, `ls c:\mappedro\readonly`, `c:\`, 0, `c:\mappedro\readonly`)
	runHcsCommand(t, c, `rm c:\mappedro\readonly`, `c:\`, 1, "")
	runHcsCommand(t, c, `cp readonly fail`, `c:\mappedro`, 1, "")
	runHcsCommand(t, c, `ls`, `c:\mappedro`, 0, `readonly`)

	// Check that read-write allows new file creation and removal
	runHcsCommand(t, c, `ls`, `c:\mappedrw`, 0, `readwrite`)
	runHcsCommand(t, c, `cp readwrite succeeds`, `c:\mappedrw`, 0, "")
	runHcsCommand(t, c, `ls`, `c:\mappedrw`, 0, "readwrite\nsucceeds")
	runHcsCommand(t, c, `rm succeeds`, `c:\mappedrw`, 0, "")
	runHcsCommand(t, c, `ls`, `c:\mappedrw`, 0, `readwrite`)
}

// Helper to launch a process in a container created through the hcsshim methods.
// At the point of calling, the container must have been successfully created.
func runHcsCommand(t *testing.T,
	c *hcs.System,
	command string,
	workdir string,
	expectedExitCode int,
	expectedOutput string) {

	if c == nil {
		t.Fatalf("requested container to start is nil!")
	}
	p, err := c.CreateProcess(&hcsshim.ProcessConfig{
		CommandLine:      command,
		WorkingDirectory: workdir,
		CreateStdInPipe:  true,
		CreateStdOutPipe: true,
		CreateStdErrPipe: true,
	})
	if err != nil {
		t.Fatalf("Failed Create Process: %s", err)

	}
	defer p.Close()
	if err := p.Wait(); err != nil {
		t.Fatalf("Failed Wait Process: %s", err)
	}
	exitCode, err := p.ExitCode()
	if err != nil {
		t.Fatalf("Failed to obtain process exit code: %s", err)
	}
	if exitCode != expectedExitCode {
		t.Fatalf("Exit code from %s wasn't %d (%d)", command, expectedExitCode, exitCode)
	}
	_, o, _, err := p.Stdio()
	if err != nil {
		t.Fatalf("Failed to get Stdio handles for process: %s", err)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(o)
	out := strings.TrimSpace(buf.String())
	if expectedOutput != "" {
		if out != expectedOutput {
			t.Fatalf("Failed to get %q from process: %q", expectedOutput, out)
		}
	}
}

// busybox is used as it has lots of layers. Exercises more code.
// Also the commands are more flexible for verification
const imageName = "busyboxw"

// Creates two temp folders used for the mounts/mapped directories
func createTestMounts(t *testing.T) (string, string) {
	// Create two temp folders for mapped directories.
	hostRWSharedDirectory := testutilities.CreateTempDir(t)
	hostROSharedDirectory := testutilities.CreateTempDir(t)
	fRW, _ := os.OpenFile(filepath.Join(hostRWSharedDirectory, "readwrite"), os.O_RDWR|os.O_CREATE, 0755)
	fRO, _ := os.OpenFile(filepath.Join(hostROSharedDirectory, "readonly"), os.O_RDWR|os.O_CREATE, 0755)
	fRW.Close()
	fRO.Close()
	return hostRWSharedDirectory, hostROSharedDirectory
}

// For calling hcsshim interface, need hcsshim.Layer built from an images layer folders
func generateShimLayersStruct(t *testing.T, imageLayers []string) []hcsshim.Layer {
	var layers []hcsshim.Layer
	for _, layerFolder := range imageLayers {
		guid, _ := wclayer.NameToGuid(filepath.Base(layerFolder))
		layers = append(layers, hcsshim.Layer{Path: layerFolder, ID: guid.String()})
	}
	return layers
}

// Argon through HCSShim interface (v1)
func TestWCOWArgonShim(t *testing.T) {
	imageLayers := testutilities.LayerFolders(t, imageName)
	argonShimMounted := false

	argonShimScratchDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(argonShimScratchDir)
	if err := wclayer.CreateScratchLayer(argonShimScratchDir, imageLayers); err != nil {
		t.Fatalf("failed to create argon scratch layer: %s", err)
	}

	hostRWSharedDirectory, hostROSharedDirectory := createTestMounts(t)
	defer os.RemoveAll(hostRWSharedDirectory)
	defer os.RemoveAll(hostROSharedDirectory)

	layers := generateShimLayersStruct(t, imageLayers)

	// For cleanup on failure
	defer func() {
		if argonShimMounted {
			hcsoci.UnmountContainerLayers(append(imageLayers, argonShimScratchDir), "", nil, hcsoci.UnmountOperationAll)
		}
	}()

	// This is a cheat but stops us re-writing exactly the same code just for test
	argonShimLocalMountPath, err := hcsoci.MountContainerLayers(append(imageLayers, argonShimScratchDir), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	argonShimMounted = true
	argonShim, err := hcsshim.CreateContainer("argon", &hcsshim.ContainerConfig{
		SystemType:      "Container",
		Name:            "argonShim",
		VolumePath:      argonShimLocalMountPath.(string),
		LayerFolderPath: argonShimScratchDir,
		Layers:          layers,
		MappedDirectories: []schema1.MappedDir{
			{
				HostPath:      hostROSharedDirectory,
				ContainerPath: `c:\mappedro`,
				ReadOnly:      true,
			},
			{
				HostPath:      hostRWSharedDirectory,
				ContainerPath: `c:\mappedrw`,
			},
		},
		HvRuntime: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	startContainer(t, argonShim)
	runShimCommands(t, argonShim)
	stopContainer(t, argonShim)
	if err := hcsoci.UnmountContainerLayers(append(imageLayers, argonShimScratchDir), "", nil, hcsoci.UnmountOperationAll); err != nil {
		t.Fatal(err)
	}
	argonShimMounted = false

}

// Xenon through HCSShim interface (v1)
func TestWCOWXenonShim(t *testing.T) {
	imageLayers := testutilities.LayerFolders(t, imageName)

	xenonShimScratchDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(xenonShimScratchDir)
	if err := wclayer.CreateScratchLayer(xenonShimScratchDir, imageLayers); err != nil {
		t.Fatalf("failed to create xenon scratch layer: %s", err)
	}

	hostRWSharedDirectory, hostROSharedDirectory := createTestMounts(t)
	defer os.RemoveAll(hostRWSharedDirectory)
	defer os.RemoveAll(hostROSharedDirectory)

	uvmImagePath, err := uvmfolder.LocateUVMFolder(imageLayers)
	if err != nil {
		t.Fatalf("LocateUVMFolder failed %s", err)
	}

	layers := generateShimLayersStruct(t, imageLayers)

	xenonShim, err := hcsshim.CreateContainer("xenon", &hcsshim.ContainerConfig{
		SystemType:      "Container",
		Name:            "xenonShim",
		LayerFolderPath: xenonShimScratchDir,
		Layers:          layers,
		HvRuntime:       &hcsshim.HvRuntime{ImagePath: filepath.Join(uvmImagePath, "UtilityVM")},
		HvPartition:     true,
		MappedDirectories: []schema1.MappedDir{
			{
				HostPath:      hostROSharedDirectory,
				ContainerPath: `c:\mappedro`,
				ReadOnly:      true,
			},
			{
				HostPath:      hostRWSharedDirectory,
				ContainerPath: `c:\mappedrw`,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	startContainer(t, xenonShim)
	runShimCommands(t, xenonShim)
	stopContainer(t, xenonShim)
}

func generateWCOWOciTestSpec(t *testing.T, imageLayers []string, scratchPath, hostRWSharedDirectory, hostROSharedDirectory string) *specs.Spec {
	return &specs.Spec{
		Windows: &specs.Windows{
			LayerFolders: append(imageLayers, scratchPath),
		},
		Mounts: []specs.Mount{
			{
				Source:      hostROSharedDirectory,
				Destination: `c:\mappedro`,
				Options:     []string{"ro"},
			},
			{
				Source:      hostRWSharedDirectory,
				Destination: `c:\mappedrw`,
			},
		},
	}
}

// Argon through HCSOCI interface (v1)
func TestWCOWArgonOciV1(t *testing.T) {
	imageLayers := testutilities.LayerFolders(t, imageName)
	argonOci1Mounted := false
	argonOci1ScratchDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(argonOci1ScratchDir)
	if err := wclayer.CreateScratchLayer(argonOci1ScratchDir, imageLayers); err != nil {
		t.Fatalf("failed to create argon scratch layer: %s", err)
	}

	hostRWSharedDirectory, hostROSharedDirectory := createTestMounts(t)
	defer os.RemoveAll(hostRWSharedDirectory)
	defer os.RemoveAll(hostROSharedDirectory)

	// For cleanup on failure
	var argonOci1Resources *hcsoci.Resources
	var argonOci1 *hcs.System
	defer func() {
		if argonOci1Mounted {
			hcsoci.ReleaseResources(argonOci1Resources, nil, true)
		}
	}()

	var err error
	spec := generateWCOWOciTestSpec(t, imageLayers, argonOci1ScratchDir, hostRWSharedDirectory, hostROSharedDirectory)
	argonOci1, argonOci1Resources, err = hcsoci.CreateContainer(
		&hcsoci.CreateOptions{
			ID:            "argonOci1",
			SchemaVersion: schemaversion.SchemaV10(),
			Spec:          spec,
		})
	if err != nil {
		t.Fatal(err)
	}
	argonOci1Mounted = true
	startContainer(t, argonOci1)
	runHcsCommands(t, argonOci1)
	stopContainer(t, argonOci1)
	if err := hcsoci.ReleaseResources(argonOci1Resources, nil, true); err != nil {
		t.Fatal(err)
	}
	argonOci1Mounted = false
}

// Xenon through HCSOCI interface (v1)
func TestWCOWXenonOciV1(t *testing.T) {
	imageLayers := testutilities.LayerFolders(t, imageName)
	xenonOci1Mounted := false

	xenonOci1ScratchDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(xenonOci1ScratchDir)
	if err := wclayer.CreateScratchLayer(xenonOci1ScratchDir, imageLayers); err != nil {
		t.Fatalf("failed to create xenon scratch layer: %s", err)
	}

	hostRWSharedDirectory, hostROSharedDirectory := createTestMounts(t)
	defer os.RemoveAll(hostRWSharedDirectory)
	defer os.RemoveAll(hostROSharedDirectory)

	// TODO: This isn't currently used.
	//	uvmImagePath, err := uvmfolder.LocateUVMFolder(imageLayers)
	//	if err != nil {
	//		t.Fatalf("LocateUVMFolder failed %s", err)
	//	}

	// For cleanup on failure
	var xenonOci1Resources *hcsoci.Resources
	var xenonOci1 *hcs.System
	defer func() {
		if xenonOci1Mounted {
			hcsoci.ReleaseResources(xenonOci1Resources, nil, true)
		}
	}()

	var err error
	spec := generateWCOWOciTestSpec(t, imageLayers, xenonOci1ScratchDir, hostRWSharedDirectory, hostROSharedDirectory)
	spec.Windows.HyperV = &specs.WindowsHyperV{}
	xenonOci1, xenonOci1Resources, err = hcsoci.CreateContainer(
		&hcsoci.CreateOptions{
			ID:            "xenonOci1",
			SchemaVersion: schemaversion.SchemaV10(),
			Spec:          spec,
		})
	if err != nil {
		t.Fatal(err)
	}
	xenonOci1Mounted = true
	startContainer(t, xenonOci1)
	runHcsCommands(t, xenonOci1)
	stopContainer(t, xenonOci1)
	if err := hcsoci.ReleaseResources(xenonOci1Resources, nil, true); err != nil {
		t.Fatal(err)
	}
	xenonOci1Mounted = false
}

// Argon through HCSOCI interface (v2)
func TestWCOWArgonOciV2(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	imageLayers := testutilities.LayerFolders(t, imageName)
	argonOci2Mounted := false

	argonOci2ScratchDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(argonOci2ScratchDir)
	if err := wclayer.CreateScratchLayer(argonOci2ScratchDir, imageLayers); err != nil {
		t.Fatalf("failed to create argon scratch layer: %s", err)
	}

	hostRWSharedDirectory, hostROSharedDirectory := createTestMounts(t)
	defer os.RemoveAll(hostRWSharedDirectory)
	defer os.RemoveAll(hostROSharedDirectory)

	// For cleanup on failure
	var argonOci2Resources *hcsoci.Resources
	var argonOci2 *hcs.System
	defer func() {
		if argonOci2Mounted {
			hcsoci.ReleaseResources(argonOci2Resources, nil, true)
		}
	}()

	var err error
	spec := generateWCOWOciTestSpec(t, imageLayers, argonOci2ScratchDir, hostRWSharedDirectory, hostROSharedDirectory)
	argonOci2, argonOci2Resources, err = hcsoci.CreateContainer(
		&hcsoci.CreateOptions{
			ID:            "argonOci2",
			SchemaVersion: schemaversion.SchemaV21(),
			Spec:          spec,
		})
	if err != nil {
		t.Fatal(err)
	}
	argonOci2Mounted = true
	startContainer(t, argonOci2)
	runHcsCommands(t, argonOci2)
	stopContainer(t, argonOci2)
	if err := hcsoci.ReleaseResources(argonOci2Resources, nil, true); err != nil {
		t.Fatal(err)
	}
	argonOci2Mounted = false

}

// Xenon through HCSOCI interface (v2)
func TestWCOWXenonOciV2(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	imageLayers := testutilities.LayerFolders(t, imageName)
	xenonOci2Mounted := false
	xenonOci2UVMCreated := false

	xenonOci2ScratchDir := testutilities.CreateTempDir(t)
	defer os.RemoveAll(xenonOci2ScratchDir)
	if err := wclayer.CreateScratchLayer(xenonOci2ScratchDir, imageLayers); err != nil {
		t.Fatalf("failed to create xenon scratch layer: %s", err)
	}

	hostRWSharedDirectory, hostROSharedDirectory := createTestMounts(t)
	defer os.RemoveAll(hostRWSharedDirectory)
	defer os.RemoveAll(hostROSharedDirectory)

	uvmImagePath, err := uvmfolder.LocateUVMFolder(imageLayers)
	if err != nil {
		t.Fatalf("LocateUVMFolder failed %s", err)
	}

	var xenonOci2Resources *hcsoci.Resources
	var xenonOci2 *hcs.System
	var xenonOci2UVM *uvm.UtilityVM
	defer func() {
		if xenonOci2Mounted {
			hcsoci.ReleaseResources(xenonOci2Resources, xenonOci2UVM, true)
		}
		if xenonOci2UVMCreated {
			xenonOci2UVM.Close()
		}
	}()

	// Create the utility VM.
	xenonOci2UVMId := "xenonOci2UVM"
	xenonOci2UVMScratchDir := testutilities.CreateTempDir(t)
	if err := wcow.CreateUVMScratch(uvmImagePath, xenonOci2UVMScratchDir, xenonOci2UVMId); err != nil {
		t.Fatalf("failed to create scratch: %s", err)
	}

	xenonOciOpts := uvm.NewDefaultOptionsWCOW(xenonOci2UVMId, "")
	xenonOciOpts.LayerFolders = append(imageLayers, xenonOci2UVMScratchDir)
	xenonOci2UVM, err = uvm.CreateWCOW(xenonOciOpts)
	if err != nil {
		t.Fatalf("Failed create UVM: %s", err)
	}
	xenonOci2UVMCreated = true
	if err := xenonOci2UVM.Start(); err != nil {
		xenonOci2UVM.Close()
		t.Fatalf("Failed start UVM: %s", err)

	}

	spec := generateWCOWOciTestSpec(t, imageLayers, xenonOci2ScratchDir, hostRWSharedDirectory, hostROSharedDirectory)
	xenonOci2, xenonOci2Resources, err = hcsoci.CreateContainer(
		&hcsoci.CreateOptions{
			ID:            "xenonOci2",
			HostingSystem: xenonOci2UVM,
			SchemaVersion: schemaversion.SchemaV21(),
			Spec:          spec,
		})
	if err != nil {
		t.Fatal(err)
	}
	xenonOci2Mounted = true
	startContainer(t, xenonOci2)
	runHcsCommands(t, xenonOci2)
	stopContainer(t, xenonOci2)
	if err := hcsoci.ReleaseResources(xenonOci2Resources, xenonOci2UVM, true); err != nil {
		t.Fatal(err)
	}
	xenonOci2Mounted = false

	// Terminate the UVM
	xenonOci2UVM.Close()
	xenonOci2UVMCreated = false
}
