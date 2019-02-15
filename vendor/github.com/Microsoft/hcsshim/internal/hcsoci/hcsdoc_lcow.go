// +build windows

package hcsoci

import (
	"encoding/json"

	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/internal/schemaversion"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

func createLCOWSpec(coi *createOptionsInternal) (*specs.Spec, error) {
	// Remarshal the spec to perform a deep copy.
	j, err := json.Marshal(coi.Spec)
	if err != nil {
		return nil, err
	}
	spec := &specs.Spec{}
	err = json.Unmarshal(j, spec)
	if err != nil {
		return nil, err
	}

	// TODO
	// Translate the mounts. The root has already been translated in
	// allocateLinuxResources.
	/*
		for i := range spec.Mounts {
			spec.Mounts[i].Source = "???"
			spec.Mounts[i].Destination = "???"
		}
	*/

	// Linux containers don't care about Windows aspects of the spec except the
	// network namespace
	spec.Windows = nil
	if coi.Spec.Windows != nil &&
		coi.Spec.Windows.Network != nil &&
		coi.Spec.Windows.Network.NetworkNamespace != "" {
		spec.Windows = &specs.Windows{
			Network: &specs.WindowsNetwork{
				NetworkNamespace: coi.Spec.Windows.Network.NetworkNamespace,
			},
		}
	}

	// Hooks are not supported (they should be run in the host)
	spec.Hooks = nil

	// Clear unsupported features
	if spec.Linux.Resources != nil {
		spec.Linux.Resources.Devices = nil
		spec.Linux.Resources.Memory = nil
		spec.Linux.Resources.Pids = nil
		spec.Linux.Resources.BlockIO = nil
		spec.Linux.Resources.HugepageLimits = nil
		spec.Linux.Resources.Network = nil
	}
	spec.Linux.Seccomp = nil

	// Clear any specified namespaces
	var namespaces []specs.LinuxNamespace
	for _, ns := range spec.Linux.Namespaces {
		switch ns.Type {
		case specs.NetworkNamespace:
		default:
			ns.Path = ""
			namespaces = append(namespaces, ns)
		}
	}
	spec.Linux.Namespaces = namespaces

	return spec, nil
}

// This is identical to hcsschema.ComputeSystem but HostedSystem is an LCOW specific type - the schema docs only include WCOW.
type linuxComputeSystem struct {
	Owner                             string                    `json:"Owner,omitempty"`
	SchemaVersion                     *hcsschema.Version        `json:"SchemaVersion,omitempty"`
	HostingSystemId                   string                    `json:"HostingSystemId,omitempty"`
	HostedSystem                      *linuxHostedSystem        `json:"HostedSystem,omitempty"`
	Container                         *hcsschema.Container      `json:"Container,omitempty"`
	VirtualMachine                    *hcsschema.VirtualMachine `json:"VirtualMachine,omitempty"`
	ShouldTerminateOnLastHandleClosed bool                      `json:"ShouldTerminateOnLastHandleClosed,omitempty"`
}

type linuxHostedSystem struct {
	SchemaVersion    *hcsschema.Version
	OciBundlePath    string
	OciSpecification *specs.Spec
}

func createLinuxContainerDocument(coi *createOptionsInternal, guestRoot string) (interface{}, error) {
	spec, err := createLCOWSpec(coi)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("hcsshim::createLinuxContainerDoc: guestRoot:%s", guestRoot)
	v2 := &linuxComputeSystem{
		Owner:                             coi.actualOwner,
		SchemaVersion:                     schemaversion.SchemaV21(),
		ShouldTerminateOnLastHandleClosed: true,
		HostingSystemId:                   coi.HostingSystem.ID(),
		HostedSystem: &linuxHostedSystem{
			SchemaVersion:    schemaversion.SchemaV21(),
			OciBundlePath:    guestRoot,
			OciSpecification: spec,
		},
	}

	return v2, nil
}
