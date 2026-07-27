//go:build e2e

// Copyright 2026 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
	kindexec "sigs.k8s.io/kind/pkg/exec"
)

// cniPlugin describes a CNI plugins release asset and its checksum per arch.
var cniPluginSHA256 = map[string]string{
	"amd64": "b98f74a0f8522f0a83867178729c1aa70f2158f90c45a2ca8fa791db1c76b303",
	"arm":   "21416880bea0541d78afaf106373d6dbb471edb92c0114fa263494fe4aec8d3b",
	"arm64": "56171987d3947707c3563db2f4001bccaf50fd63468611b9f3cbecb1375ee7ec",
}

const cniPluginsVersion = "v1.9.1"

// installCNIPlugins downloads and extracts the CNI plugins into /opt/cni/bin,
// mirroring the former install_cni_plugins bash helper. It is a host-side step
// (kind nodes bind-mount the host's /opt/cni/bin is not used; instead the kind
// image ships plugins, but the flannel manifest relies on the host set), kept
// for parity with the previous suite.
func installCNIPlugins(arch string) error {
	sum, ok := cniPluginSHA256[arch]
	if !ok {
		return fmt.Errorf("unsupported ARCH for CNI plugins: %s", arch)
	}
	tgz := fmt.Sprintf("cni-plugins-linux-%s-%s.tgz", arch, cniPluginsVersion)
	url := fmt.Sprintf("https://github.com/containernetworking/plugins/releases/download/%s/%s", cniPluginsVersion, tgz)

	dst := "/tmp/" + tgz
	if err := downloadFile(url, dst); err != nil {
		return fmt.Errorf("downloading CNI plugins: %w", err)
	}
	defer os.Remove(dst)

	if err := verifySHA256(dst, sum); err != nil {
		return err
	}

	if out, err := exec.Command("sudo", "mkdir", "-p", "/opt/cni/bin").CombinedOutput(); err != nil {
		return fmt.Errorf("mkdir /opt/cni/bin: %w: %s", err, out)
	}
	if out, err := exec.Command("sudo", "tar", "-C", "/opt/cni/bin", "-xzf", dst).CombinedOutput(); err != nil {
		return fmt.Errorf("extracting CNI plugins: %w: %s", err, out)
	}
	return nil
}

func downloadFile(url, dst string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s downloading %s", resp.Status, url)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func verifySHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", path, got, want)
	}
	return nil
}

// kindCluster wraps a running kind cluster and its Kubernetes client.
type kindCluster struct {
	provider *cluster.Provider
	name     string
	restCfg  *rest.Config
	client   *kubernetes.Clientset
	applied  []appliedObject
}

// createCluster creates a fresh kind cluster, loads the flannel and iperf3
// images into it and returns a ready-to-use client. It mirrors the former
// setup() bash function.
func createCluster() (*kindCluster, error) {
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))

	By("creating kind cluster " + kindClusterName)
	if err := provider.Create(
		kindClusterName,
		cluster.CreateWithConfigFile("kind-config.yaml"),
		cluster.CreateWithWaitForReady(5*time.Minute),
	); err != nil {
		return nil, fmt.Errorf("creating kind cluster: %w", err)
	}

	kc := &kindCluster{provider: provider, name: kindClusterName}

	By("loading images into kind cluster")
	for _, img := range []string{flannelImage, iperf3Image} {
		if err := kc.loadImage(img); err != nil {
			return kc, fmt.Errorf("loading image %s: %w", img, err)
		}
	}

	kubeconfig, err := provider.KubeConfig(kindClusterName, false)
	if err != nil {
		return kc, fmt.Errorf("getting kubeconfig: %w", err)
	}
	restCfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return kc, fmt.Errorf("building rest config: %w", err)
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return kc, fmt.Errorf("building clientset: %w", err)
	}
	kc.restCfg = restCfg
	kc.client = client
	return kc, nil
}

// loadImage saves a docker image to a tar stream and imports it into every kind
// node, equivalent to `kind load docker-image`.
func (kc *kindCluster) loadImage(image string) error {
	nodeList, err := kc.provider.ListInternalNodes(kc.name)
	if err != nil {
		return err
	}

	save := exec.Command("docker", "save", image)
	stdout, err := save.StdoutPipe()
	if err != nil {
		return err
	}
	if err := save.Start(); err != nil {
		return err
	}

	// Buffer the archive so it can be replayed to each node.
	data, err := io.ReadAll(stdout)
	if err != nil {
		return err
	}
	if err := save.Wait(); err != nil {
		return fmt.Errorf("docker save %s: %w", image, err)
	}

	for _, n := range nodeList {
		if err := nodeutils.LoadImageArchive(n, bytes.NewReader(data)); err != nil {
			return fmt.Errorf("importing image into node %s: %w", n, err)
		}
	}
	return nil
}

// delete tears the cluster down, mirroring teardown().
func (kc *kindCluster) delete() error {
	if kc == nil || kc.provider == nil {
		return nil
	}
	By("deleting kind cluster " + kc.name)
	return kc.provider.Delete(kc.name, "")
}

// nodeByName returns the kind node object for a given container name.
func (kc *kindCluster) nodeByName(name string) (nodes.Node, error) {
	nodeList, err := kc.provider.ListNodes(kc.name)
	if err != nil {
		return nil, err
	}
	for _, n := range nodeList {
		if n.String() == name {
			return n, nil
		}
	}
	return nil, fmt.Errorf("node %s not found", name)
}

// execOnNode runs a command inside a kind node container (docker exec
// equivalent) and returns its combined stdout.
func (kc *kindCluster) execOnNode(nodeName string, cmd ...string) (string, error) {
	n, err := kc.nodeByName(nodeName)
	if err != nil {
		return "", err
	}
	out, err := kindexec.Output(n.Command(cmd[0], cmd[1:]...))
	return string(out), err
}
