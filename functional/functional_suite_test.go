//go:build functional

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

// Package functional contains the docker-based functional test suite for
// flannel. It replaces the former bash_unit suites (dist/functional-test.sh and
// dist/functional-test-k8s.sh) with a native Ginkgo v2 + Gomega suite.
//
// Run with:
//
//	go test -tags functional -timeout 60m -v ./functional/...
//
// The following environment variables tune the run (defaults in parentheses):
//
//	ARCH                  target architecture, gates udp/amd64-only tests (amd64)
//	TAG                   git-describe tag used to build FLANNEL_DOCKER_IMAGE
//	FLANNEL_DOCKER_IMAGE  full flannel image ref (quay.io/coreos/flannel:$TAG-$ARCH)
//	FLANNEL_NET           IPv4 pod network                              (10.10.0.0/16)
//	ETCD_IMG              etcd docker image       (quay.io/coreos/etcd:v3.6.2)
//	ETCD_LOCATION         etcd binary inside the image                  (etcd)
//	K8S_VERSION           kubernetes version for the apiserver          (1.32.6)
//	HYPERKUBE_IMG         hyperkube image base    (docker.io/rancher/hyperkube)
//	HYPERKUBE_CMD         hyperkube binary override                     ("")
//	HYPERKUBE_APISERVER_CMD  api-server sub-command                     (kube-apiserver)
package functional

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// suite-wide configuration derived from the environment.
var (
	arch               string
	flannelDockerImage string
	flannelNet         string
	etcdImg            string
	etcdctlImg         string
	etcdLocation       string
	k8sVersion         string
	hyperkubeImg       string
	hyperkubeCmd       string
	hyperkubeAPICmd    string
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "flannel docker functional suite")
}

var _ = BeforeSuite(func() {
	arch = env("ARCH", "amd64")
	flannelNet = env("FLANNEL_NET", "10.10.0.0/16")
	etcdLocation = env("ETCD_LOCATION", "etcd")
	k8sVersion = env("K8S_VERSION", "1.32.6")
	hyperkubeImg = env("HYPERKUBE_IMG", "docker.io/rancher/hyperkube")
	hyperkubeCmd = env("HYPERKUBE_CMD", "")
	hyperkubeAPICmd = env("HYPERKUBE_APISERVER_CMD", "kube-apiserver")

	base := env("ETCD_IMG", "quay.io/coreos/etcd:v3.6.2")
	etcdImg = base
	etcdctlImg = "quay.io/coreos/etcd:v3.6.2"
	switch arch {
	case "ppc64le":
		etcdImg += "-ppc64le"
		etcdctlImg += "-ppc64le"
	case "arm64":
		etcdImg += "-arm64"
		etcdctlImg += "-arm64"
	}

	flannelDockerImage = os.Getenv("FLANNEL_DOCKER_IMAGE")
	if flannelDockerImage == "" {
		tag := os.Getenv("TAG")
		Expect(tag).NotTo(BeEmpty(),
			"either FLANNEL_DOCKER_IMAGE or TAG must be set")
		flannelDockerImage = fmt.Sprintf("quay.io/coreos/flannel:%s-%s", tag, arch)
	}

	By(fmt.Sprintf("using flannel image %s (arch %s)", flannelDockerImage, arch))
})
