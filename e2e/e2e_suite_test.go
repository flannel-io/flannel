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
// Package e2e contains the kind-based end-to-end test suite for flannel.
//
// It replaces the former bash_unit suite (run-e2e-tests.sh) with a native
// Ginkgo v2 + Gomega suite that drives Kubernetes through client-go and manages
// the kind cluster through the sigs.k8s.io/kind library.
//
// Run with:
//
//	go test -tags e2e ./e2e/... -v
//
// The following environment variables tune the run (defaults in parentheses):
//
//	ARCH             target architecture, gates udp/amd64-only tests (amd64)
//	TAG              image tag used to build FLANNEL_IMAGE               (git describe)
//	FLANNEL_IMAGE    full flannel image ref (quay.io/coreos/flannel:$TAG-$ARCH)
//	FLANNEL_NET      IPv4 pod network                                    (10.42.0.0/16)
//	IPERF3_IMAGE     iperf3 image reference                              (iperf3:latest)
//	MULTITOOL_IMAGE  connectivity test image           (wbitt/network-multitool:alpine-extra)
package e2e

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	kindClusterName  = "e2e"
	kindControlPlane = kindClusterName + "-control-plane"
	kindWorker       = kindClusterName + "-worker"

	flannelNamespace = "kube-flannel"
)

// suite-wide configuration derived from the environment.
var (
	arch           string
	flannelImage   string
	iperf3Image    string
	multitoolImage string
	flannelNet     string
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "flannel kind e2e suite")
}

var _ = BeforeSuite(func() {
	arch = env("ARCH", "amd64")
	flannelNet = env("FLANNEL_NET", "10.42.0.0/16")
	iperf3Image = env("IPERF3_IMAGE", "iperf3:latest")
	multitoolImage = env("MULTITOOL_IMAGE", "wbitt/network-multitool:alpine-extra")

	flannelImage = os.Getenv("FLANNEL_IMAGE")
	if flannelImage == "" {
		tag := os.Getenv("TAG")
		Expect(tag).NotTo(BeEmpty(),
			"either FLANNEL_IMAGE or TAG must be set so the flannel image can be resolved")
		flannelImage = fmt.Sprintf("quay.io/coreos/flannel:%s-%s", tag, arch)
	}

	By(fmt.Sprintf("using flannel image %s (arch %s)", flannelImage, arch))

	// CNI plugins must be present on the host so the kind nodes can wire pods.
	Expect(installCNIPlugins(arch)).To(Succeed(), "installing CNI plugins")
})
