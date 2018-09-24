package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	winio "github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcsoci"
	"github.com/Microsoft/hcsshim/internal/regstate"
	"github.com/Microsoft/hcsshim/internal/uvm"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var errContainerStopped = errors.New("container is stopped")

type persistedState struct {
	ID             string
	SandboxID      string
	HostID         string
	Bundle         string
	Created        time.Time
	Rootfs         string
	Spec           *specs.Spec
	RequestedNetNS string
	IsHost         bool
	UniqueID       guid.GUID
	HostUniqueID   guid.GUID
}

type containerStatus string

const (
	containerRunning containerStatus = "running"
	containerStopped containerStatus = "stopped"
	containerCreated containerStatus = "created"
	containerPaused  containerStatus = "paused"
	containerUnknown containerStatus = "unknown"

	keyState     = "state"
	keyResources = "resources"
	keyShimPid   = "shim"
	keyInitPid   = "pid"
	keyNetNS     = "netns"
)

type container struct {
	persistedState
	ShimPid   int
	hc        *hcs.System
	resources *hcsoci.Resources
}

func getErrorFromPipe(pipe io.Reader, p *os.Process) error {
	serr, err := ioutil.ReadAll(pipe)
	if err != nil {
		return err
	}

	if bytes.Equal(serr, shimSuccess) {
		return nil
	}

	extra := ""
	if p != nil {
		p.Kill()
		state, err := p.Wait()
		if err != nil {
			panic(err)
		}
		extra = fmt.Sprintf(", exit code %d", state.Sys().(syscall.WaitStatus).ExitCode)
	}
	if len(serr) == 0 {
		return fmt.Errorf("unknown shim failure%s", extra)
	}

	return errors.New(string(serr))
}

func startProcessShim(id, pidFile, logFile string, spec *specs.Process) (_ *os.Process, err error) {
	// Ensure the stdio handles inherit to the child process. This isn't undone
	// after the StartProcess call because the caller never launches another
	// process before exiting.
	for _, f := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
		err = windows.SetHandleInformation(windows.Handle(f.Fd()), windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT)
		if err != nil {
			return nil, err
		}
	}

	args := []string{
		"--stdin", strconv.Itoa(int(os.Stdin.Fd())),
		"--stdout", strconv.Itoa(int(os.Stdout.Fd())),
		"--stderr", strconv.Itoa(int(os.Stderr.Fd())),
	}
	if spec != nil {
		args = append(args, "--exec")
	}
	args = append(args, id)
	return launchShim("shim", pidFile, logFile, args, spec)
}

func launchShim(cmd, pidFile, logFile string, args []string, data interface{}) (_ *os.Process, err error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}

	// Create a pipe to use as stderr for the shim process. This is used to
	// retrieve early error information, up to the point that the shim is ready
	// to launch a process in the container.
	rp, wp, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer rp.Close()
	defer wp.Close()

	// Create a pipe to send the data, if one is provided.
	var rdatap, wdatap *os.File
	if data != nil {
		rdatap, wdatap, err = os.Pipe()
		if err != nil {
			return nil, err
		}
		defer rdatap.Close()
		defer wdatap.Close()
	}

	var log *os.File
	fullargs := []string{os.Args[0]}
	if logFile != "" {
		log, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0666)
		if err != nil {
			return nil, err
		}
		defer log.Close()

		fullargs = append(fullargs, "--log-format", logFormat)
		if logrus.GetLevel() == logrus.DebugLevel {
			fullargs = append(fullargs, "--debug")
		}
	}
	fullargs = append(fullargs, cmd)
	fullargs = append(fullargs, args...)
	attr := &os.ProcAttr{
		Files: []*os.File{rdatap, wp, log},
	}
	p, err := os.StartProcess(executable, fullargs, attr)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			p.Kill()
		}
	}()

	wp.Close()

	// Write the data if provided.
	if data != nil {
		rdatap.Close()
		dataj, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		_, err = wdatap.Write(dataj)
		if err != nil {
			return nil, err
		}
		wdatap.Close()
	}

	err = getErrorFromPipe(rp, p)
	if err != nil {
		return nil, err
	}

	if pidFile != "" {
		if err = createPidFile(pidFile, p.Pid); err != nil {
			return nil, err
		}
	}

	return p, nil
}

func parseSandboxAnnotations(spec *specs.Spec) (string, bool) {
	a := spec.Annotations
	var t, id string
	if t = a["io.kubernetes.cri.container-type"]; t != "" {
		id = a["io.kubernetes.cri.sandbox-id"]
	} else if t = a["io.kubernetes.cri-o.ContainerType"]; t != "" {
		id = a["io.kubernetes.cri-o.SandboxID"]
	} else if t = a["io.kubernetes.docker.type"]; t != "" {
		id = a["io.kubernetes.sandbox.id"]
		if t == "podsandbox" {
			t = "sandbox"
		}
	}
	if t == "container" {
		return id, false
	}
	if t == "sandbox" {
		return id, true
	}
	return "", false
}

func (c *container) startVMShim(logFile string, consolePipe string) (*os.Process, error) {
	opts := &uvm.UVMOptions{
		ID:          vmID(c.ID),
		ConsolePipe: consolePipe,
	}
	if c.Spec.Windows != nil {
		opts.Resources = c.Spec.Windows.Resources
	}

	if c.Spec.Linux != nil {
		opts.OperatingSystem = "linux"
	} else {
		opts.OperatingSystem = "windows"
		layers := make([]string, len(c.Spec.Windows.LayerFolders))
		for i, f := range c.Spec.Windows.LayerFolders {
			if i == len(c.Spec.Windows.LayerFolders)-1 {
				f = filepath.Join(f, "vm")
				err := os.MkdirAll(f, 0)
				if err != nil {
					return nil, err
				}
			}
			layers[i] = f
		}
		opts.LayerFolders = layers
	}
	return launchShim("vmshim", "", logFile, []string{c.VMPipePath()}, opts)
}

type containerConfig struct {
	ID                     string
	HostID                 string
	PidFile                string
	ShimLogFile, VMLogFile string
	Spec                   *specs.Spec
	VMConsolePipe          string
}

func createContainer(cfg *containerConfig) (_ *container, err error) {
	// Store the container information in a volatile registry key.
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	vmisolated := cfg.Spec.Linux != nil || (cfg.Spec.Windows != nil && cfg.Spec.Windows.HyperV != nil)

	sandboxID, isSandbox := parseSandboxAnnotations(cfg.Spec)
	hostID := cfg.HostID
	if isSandbox {
		if sandboxID != cfg.ID {
			return nil, errors.New("sandbox ID must match ID")
		}
	} else if sandboxID != "" {
		// Validate that the sandbox container exists.
		sandbox, err := getContainer(sandboxID, false)
		if err != nil {
			return nil, err
		}
		defer sandbox.Close()
		if sandbox.SandboxID != sandboxID {
			return nil, fmt.Errorf("container %s is not a sandbox", sandboxID)
		}
		if hostID == "" {
			// Use the sandbox's host.
			hostID = sandbox.HostID
		} else if sandbox.HostID == "" {
			return nil, fmt.Errorf("sandbox container %s is not running in a VM host, but host %s was specified", sandboxID, hostID)
		} else if hostID != sandbox.HostID {
			return nil, fmt.Errorf("sandbox container %s has a different host %s from the requested host %s", sandboxID, sandbox.HostID, hostID)
		}
		if vmisolated && hostID == "" {
			return nil, fmt.Errorf("container %s is not a VM isolated sandbox", sandboxID)
		}
	}

	uniqueID := guid.New()

	newvm := false
	var hostUniqueID guid.GUID
	if hostID != "" {
		host, err := getContainer(hostID, false)
		if err != nil {
			return nil, err
		}
		defer host.Close()
		if !host.IsHost {
			return nil, fmt.Errorf("host container %s is not a VM host", hostID)
		}
		hostUniqueID = host.UniqueID
	} else if vmisolated && (isSandbox || cfg.Spec.Linux != nil) {
		hostID = cfg.ID
		newvm = true
		hostUniqueID = uniqueID
	}

	// Make absolute the paths in Root.Path and Windows.LayerFolders.
	rootfs := ""
	if cfg.Spec.Root != nil {
		rootfs = cfg.Spec.Root.Path
		if rootfs != "" && !filepath.IsAbs(rootfs) && !strings.HasPrefix(rootfs, `\\?\`) {
			rootfs = filepath.Join(cwd, rootfs)
			cfg.Spec.Root.Path = rootfs
		}
	}

	netNS := ""
	if cfg.Spec.Windows != nil {
		for i, f := range cfg.Spec.Windows.LayerFolders {
			if !filepath.IsAbs(f) && !strings.HasPrefix(rootfs, `\\?\`) {
				cfg.Spec.Windows.LayerFolders[i] = filepath.Join(cwd, f)
			}
		}

		// Determine the network namespace to use.
		if cfg.Spec.Windows.Network != nil && cfg.Spec.Windows.Network.NetworkSharedContainerName != "" {
			err = stateKey.Get(cfg.Spec.Windows.Network.NetworkSharedContainerName, keyNetNS, &netNS)
			if err != nil {
				if _, ok := err.(*regstate.NoStateError); !ok {
					return nil, err
				}
			}
		}
	}

	// Store the initial container state in the registry so that the delete
	// command can clean everything up if something goes wrong.
	c := &container{
		persistedState: persistedState{
			ID:             cfg.ID,
			Bundle:         cwd,
			Rootfs:         rootfs,
			Created:        time.Now(),
			Spec:           cfg.Spec,
			SandboxID:      sandboxID,
			HostID:         hostID,
			IsHost:         newvm,
			RequestedNetNS: netNS,
			UniqueID:       uniqueID,
			HostUniqueID:   hostUniqueID,
		},
	}
	err = stateKey.Create(cfg.ID, keyState, &c.persistedState)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			c.Remove()
		}
	}()

	// Start a VM if necessary.
	if newvm {
		shim, err := c.startVMShim(cfg.VMLogFile, cfg.VMConsolePipe)
		if err != nil {
			return nil, err
		}
		shim.Release()
	}

	if c.HostID != "" {
		// Call to the VM shim process to create the container. This is done so
		// that the VM process can keep track of the VM's virtual hardware
		// resource use.
		err = c.issueVMRequest(opCreateContainer)
		if err != nil {
			return nil, err
		}
		c.hc, err = hcs.OpenComputeSystem(cfg.ID)
		if err != nil {
			return nil, err
		}
	} else {
		// Create the container directly from this process.
		err = createContainerInHost(c, nil)
		if err != nil {
			return nil, err
		}
	}

	// Create the shim process for the container.
	err = startContainerShim(c, cfg.PidFile, cfg.ShimLogFile)
	if err != nil {
		if e := c.Kill(); e == nil {
			c.Remove()
		}
		return nil, err
	}

	return c, nil
}

func (c *container) ShimPipePath() string {
	return safePipePath("runhcs-shim-" + c.UniqueID.String())
}

func (c *container) VMPipePath() string {
	return safePipePath("runhcs-vm-" + c.HostUniqueID.String())
}

func (c *container) VMIsolated() bool {
	return c.HostID != ""
}

func (c *container) unmountInHost(vm *uvm.UtilityVM, all bool) error {
	resources := &hcsoci.Resources{}
	err := stateKey.Get(c.ID, keyResources, resources)
	if _, ok := err.(*regstate.NoStateError); ok {
		return nil
	}
	if err != nil {
		return err
	}
	err = hcsoci.ReleaseResources(resources, vm, all)
	if err != nil {
		stateKey.Set(c.ID, keyResources, resources)
		return err
	}

	err = stateKey.Clear(c.ID, keyResources)
	if err != nil {
		return err
	}
	return nil
}

func (c *container) Unmount(all bool) error {
	if c.VMIsolated() {
		op := opUnmountContainerDiskOnly
		if all {
			op = opUnmountContainer
		}
		err := c.issueVMRequest(op)
		if err != nil {
			if _, ok := err.(*noVMError); ok {
				logrus.Warnf("did not unmount resources for container %s because VM shim for %s could not be contacted", c.ID, c.HostID)
			} else {
				return err
			}
		}
	} else {
		c.unmountInHost(nil, false)
	}
	return nil
}

func createContainerInHost(c *container, vm *uvm.UtilityVM) (err error) {
	if c.hc != nil {
		return errors.New("container already created")
	}

	// Create the container without starting it.
	opts := &hcsoci.CreateOptions{
		ID:               c.ID,
		Spec:             c.Spec,
		HostingSystem:    vm,
		NetworkNamespace: c.RequestedNetNS,
	}
	vmid := ""
	if vm != nil {
		vmid = vm.ID()
	}
	logrus.Infof("creating container %s (VM: '%s')", c.ID, vmid)
	hc, resources, err := hcsoci.CreateContainer(opts)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			hc.Terminate()
			hc.Wait()
			hcsoci.ReleaseResources(resources, vm, true)
		}
	}()

	// Record the network namespace to support namespace sharing by container ID.
	if resources.NetNS() != "" {
		err = stateKey.Set(c.ID, keyNetNS, resources.NetNS)
		if err != nil {
			return err
		}
	}

	err = stateKey.Set(c.ID, keyResources, resources)
	if err != nil {
		return err
	}
	c.hc = hc
	return nil
}

func startContainerShim(c *container, pidFile, logFile string) error {
	// Launch a shim process to later execute a process in the container.
	shim, err := startProcessShim(c.ID, pidFile, logFile, nil)
	if err != nil {
		return err
	}
	defer shim.Release()
	defer func() {
		if err != nil {
			shim.Kill()
		}
	}()

	c.ShimPid = shim.Pid
	err = stateKey.Set(c.ID, keyShimPid, shim.Pid)
	if err != nil {
		return err
	}

	if pidFile != "" {
		if err = createPidFile(pidFile, shim.Pid); err != nil {
			return err
		}
	}

	return nil
}

func (c *container) Close() error {
	if c.hc == nil {
		return nil
	}
	return c.hc.Close()
}

func (c *container) Exec() error {
	err := c.hc.Start()
	if err != nil {
		return err
	}

	if c.Spec.Process == nil {
		return nil
	}

	// Alert the shim that the container is ready.
	pipe, err := winio.DialPipe(c.ShimPipePath(), nil)
	if err != nil {
		return err
	}
	defer pipe.Close()

	shim, err := os.FindProcess(c.ShimPid)
	if err != nil {
		return err
	}
	defer shim.Release()

	err = getErrorFromPipe(pipe, shim)
	if err != nil {
		return err
	}

	return nil
}

func getContainer(id string, notStopped bool) (*container, error) {
	var c container
	err := stateKey.Get(id, keyState, &c.persistedState)
	if err != nil {
		return nil, err
	}
	err = stateKey.Get(id, keyShimPid, &c.ShimPid)
	if err != nil {
		if _, ok := err.(*regstate.NoStateError); !ok {
			return nil, err
		}
		c.ShimPid = -1
	}
	if notStopped && c.ShimPid == 0 {
		return nil, errContainerStopped
	}

	hc, err := hcs.OpenComputeSystem(c.ID)
	if err == nil {
		c.hc = hc
	} else if !hcs.IsNotExist(err) {
		return nil, err
	} else if notStopped {
		return nil, errContainerStopped
	}

	return &c, nil
}

func (c *container) Remove() error {
	// Unmount any layers or mapped volumes.
	err := c.Unmount(!c.IsHost)
	if err != nil {
		return err
	}

	// Follow kata's example and delay tearing down the VM until the owning
	// container is removed.
	if c.IsHost {
		vm, err := hcs.OpenComputeSystem(vmID(c.ID))
		if err == nil {
			if err := vm.Terminate(); hcs.IsPending(err) {
				vm.Wait()
			}
		}
	}
	return stateKey.Remove(c.ID)
}

func (c *container) Kill() error {
	if c.hc == nil {
		return nil
	}
	err := c.hc.Terminate()
	if hcs.IsPending(err) {
		err = c.hc.Wait()
	}
	if hcs.IsAlreadyStopped(err) {
		err = nil
	}
	return err
}

func (c *container) Status() (containerStatus, error) {
	if c.hc == nil || c.ShimPid == 0 {
		return containerStopped, nil
	}
	props, err := c.hc.Properties()
	if err != nil {
		return "", err
	}
	state := containerUnknown
	switch props.State {
	case "", "Created":
		state = containerCreated
	case "Running":
		state = containerRunning
	case "Paused":
		state = containerPaused
	case "Stopped":
		state = containerStopped
	}
	return state, nil
}
