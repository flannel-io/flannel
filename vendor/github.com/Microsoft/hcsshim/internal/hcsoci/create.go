// +build windows

package hcsoci

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/internal/schemaversion"
	"github.com/Microsoft/hcsshim/internal/uvm"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// CreateOptions are the set of fields used to call CreateContainer().
// Note: In the spec, the LayerFolders must be arranged in the same way in which
// moby configures them: layern, layern-1,...,layer2,layer1,scratch
// where layer1 is the base read-only layer, layern is the top-most read-only
// layer, and scratch is the RW layer. This is for historical reasons only.
type CreateOptions struct {

	// Common parameters
	ID               string             // Identifier for the container
	Owner            string             // Specifies the owner. Defaults to executable name.
	Spec             *specs.Spec        // Definition of the container or utility VM being created
	SchemaVersion    *hcsschema.Version // Requested Schema Version. Defaults to v2 for RS5, v1 for RS1..RS4
	HostingSystem    *uvm.UtilityVM     // Utility or service VM in which the container is to be created.
	NetworkNamespace string             // Host network namespace to use (overrides anything in the spec)

	// This is an advanced debugging parameter. It allows for diagnosibility by leaving a containers
	// resources allocated in case of a failure. Thus you would be able to use tools such as hcsdiag
	// to look at the state of a utility VM to see what resources were allocated. Obviously the caller
	// must a) not tear down the utility VM on failure (or pause in some way) and b) is responsible for
	// performing the ReleaseResources() call themselves.
	DoNotReleaseResourcesOnFailure bool
}

// createOptionsInternal is the set of user-supplied create options, but includes internal
// fields for processing the request once user-supplied stuff has been validated.
type createOptionsInternal struct {
	*CreateOptions

	actualSchemaVersion    *hcsschema.Version // Calculated based on Windows build and optional caller-supplied override
	actualID               string             // Identifier for the container
	actualOwner            string             // Owner for the container
	actualNetworkNamespace string
}

// CreateContainer creates a container. It can cope with a  wide variety of
// scenarios, including v1 HCS schema calls, as well as more complex v2 HCS schema
// calls. Note we always return the resources that have been allocated, even in the
// case of an error. This provides support for the debugging option not to
// release the resources on failure, so that the client can make the necessary
// call to release resources that have been allocated as part of calling this function.
func CreateContainer(createOptions *CreateOptions) (_ *hcs.System, _ *Resources, err error) {
	logrus.Debugf("hcsshim::CreateContainer options: %+v", createOptions)

	coi := &createOptionsInternal{
		CreateOptions: createOptions,
		actualID:      createOptions.ID,
		actualOwner:   createOptions.Owner,
	}

	// Defaults if omitted by caller.
	if coi.actualID == "" {
		coi.actualID = guid.New().String()
	}
	if coi.actualOwner == "" {
		coi.actualOwner = filepath.Base(os.Args[0])
	}

	if coi.Spec == nil {
		return nil, nil, fmt.Errorf("Spec must be supplied")
	}

	if coi.HostingSystem != nil {
		// By definition, a hosting system can only be supplied for a v2 Xenon.
		coi.actualSchemaVersion = schemaversion.SchemaV21()
	} else {
		coi.actualSchemaVersion = schemaversion.DetermineSchemaVersion(coi.SchemaVersion)
		logrus.Debugf("hcsshim::CreateContainer using schema %s", schemaversion.String(coi.actualSchemaVersion))
	}

	resources := &Resources{}
	defer func() {
		if err != nil {
			if !coi.DoNotReleaseResourcesOnFailure {
				ReleaseResources(resources, coi.HostingSystem, true)
			}
		}
	}()

	if coi.HostingSystem != nil {
		n := coi.HostingSystem.ContainerCounter()
		if coi.Spec.Linux != nil {
			resources.containerRootInUVM = "/run/gcs/c/" + strconv.FormatUint(n, 16)
		} else {
			resources.containerRootInUVM = `C:\c\` + strconv.FormatUint(n, 16)
		}
	}

	// Create a network namespace if necessary.
	if coi.Spec.Windows != nil &&
		coi.Spec.Windows.Network != nil &&
		schemaversion.IsV21(coi.actualSchemaVersion) {

		if coi.NetworkNamespace != "" {
			resources.netNS = coi.NetworkNamespace
		} else {
			err := createNetworkNamespace(coi, resources)
			if err != nil {
				return nil, resources, err
			}
		}
		coi.actualNetworkNamespace = resources.netNS
		if coi.HostingSystem != nil {
			endpoints, err := getNamespaceEndpoints(coi.actualNetworkNamespace)
			if err != nil {
				return nil, resources, err
			}
			err = coi.HostingSystem.AddNetNS(coi.actualNetworkNamespace, endpoints)
			if err != nil {
				return nil, resources, err
			}
			resources.addedNetNSToVM = true
		}
	}

	var hcsDocument interface{}
	logrus.Debugf("hcsshim::CreateContainer allocating resources")
	if coi.Spec.Linux != nil {
		if schemaversion.IsV10(coi.actualSchemaVersion) {
			return nil, resources, errors.New("LCOW v1 not supported")
		}
		logrus.Debugf("hcsshim::CreateContainer allocateLinuxResources")
		err = allocateLinuxResources(coi, resources)
		if err != nil {
			logrus.Debugf("failed to allocateLinuxResources %s", err)
			return nil, resources, err
		}
		hcsDocument, err = createLinuxContainerDocument(coi, resources.containerRootInUVM)
		if err != nil {
			logrus.Debugf("failed createHCSContainerDocument %s", err)
			return nil, resources, err
		}
	} else {
		err = allocateWindowsResources(coi, resources)
		if err != nil {
			logrus.Debugf("failed to allocateWindowsResources %s", err)
			return nil, resources, err
		}
		logrus.Debugf("hcsshim::CreateContainer creating container document")
		hcsDocument, err = createWindowsContainerDocument(coi)
		if err != nil {
			logrus.Debugf("failed createHCSContainerDocument %s", err)
			return nil, resources, err
		}
	}

	logrus.Debugf("hcsshim::CreateContainer creating compute system")
	system, err := hcs.CreateComputeSystem(coi.actualID, hcsDocument)
	if err != nil {
		logrus.Debugf("failed to CreateComputeSystem %s", err)
		return nil, resources, err
	}
	return system, resources, err
}
