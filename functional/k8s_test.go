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
package functional

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// k8sManifestPath is the kube-flannel.yml shipped in the repo.
var k8sManifestPath = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "Documentation", "kube-flannel.yml")
}()

// opensslCNF is the openssl.cnf template used for the apiserver cert.
const opensslCNF = `[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_req]
basicConstraints = critical, CA:FALSE
keyUsage = critical, nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = kubernetes
DNS.2 = kubernetes.default
DNS.3 = kubernetes.default.svc
DNS.4 = kubernetes.default.svc.cluster
DNS.5 = kubernetes.default.svc.cluster.local
IP.1 = {{.DockerIP}}
IP.2 = 127.0.0.1
`

// node1YAML and node2YAML are the two Node manifests created in the k8s suite.
const node1YAML = `apiVersion: v1
kind: Node
metadata:
  name: flannel1
  annotations:
    dummy: value
spec:
  podCIDR: 10.10.1.0/24
`

const node2YAML = `apiVersion: v1
kind: Node
metadata:
  name: flannel2
  annotations:
    dummy: value
spec:
  podCIDR: 10.10.2.0/24
`

var _ = Describe("kube apiserver", Ordered, func() {
	var (
		etcdEndpt string
		k8sEndpt  string
		pkiDir    string // temp dir holding generated certs
	)

	BeforeAll(func() {
		dockerIP, err := docker0IP()
		Expect(err).NotTo(HaveOccurred(), "getting docker0 IP")
		etcdEndpt = "http://" + dockerIP + ":2379"
		k8sEndpt = "https://" + dockerIP + ":6443"

		// Start plain etcd (no TLS).
		dockerRm("flannel-e2e-test-etcd")
		_, err = dockerRun(
			"--name=flannel-e2e-test-etcd", "-d",
			"-p", "2379:2379",
			"-e", "ETCD_UNSUPPORTED_ARCH="+arch,
			etcdImg, etcdLocation,
			"--listen-client-urls", "http://0.0.0.0:2379",
			"--advertise-client-urls", etcdEndpt,
		)
		Expect(err).NotTo(HaveOccurred(), "starting etcd")
		time.Sleep(time.Second) // give etcd a moment to be ready

		// Generate PKI.
		pkiDir, err = generateKubePKI(dockerIP)
		Expect(err).NotTo(HaveOccurred(), "generating kube PKI")

		// Start the kube-apiserver.
		dockerRm("flannel-e2e-k8s-apiserver")
		apiserverCmd := append(strings.Fields(hyperkubeCmd), hyperkubeAPICmd)
		apiserverCmd = append(apiserverCmd,
			"--etcd-servers="+etcdEndpt,
			"--bind-address="+dockerIP,
			"--client-ca-file=/var/lib/kubernetes/pki/ca.crt",
			"--enable-admission-plugins=NodeRestriction,ServiceAccount",
			"--service-account-key-file=/var/lib/kubernetes/pki/service-account.crt",
			"--service-account-signing-key-file=/var/lib/kubernetes/pki/service-account.key",
			"--service-account-issuer=https://kubernetes.default.svc.local",
			"--tls-cert-file=/var/lib/kubernetes/pki/kube-apiserver.crt",
			"--tls-private-key-file=/var/lib/kubernetes/pki/kube-apiserver.key",
			"--service-cluster-ip-range=10.101.0.0/16",
			"--allow-privileged",
		)
		runArgs := []string{
			"-d", "--net=host",
			"-v", pkiDir + ":/var/lib/kubernetes",
			"--name", "flannel-e2e-k8s-apiserver",
			hyperkubeImg + ":v" + k8sVersion + "-rancher1",
		}
		runArgs = append(runArgs, apiserverCmd...)
		_, err = dockerRun(runArgs...)
		Expect(err).NotTo(HaveOccurred(), "starting kube-apiserver")
		time.Sleep(time.Second)

		// Build kubeconfig.
		Expect(setupKubeconfig(dockerIP)).To(Succeed())

		// Create Node objects (retry until apiserver is ready).
		By("creating Node flannel1")
		Eventually(func() error {
			_, err := dockerExecI("flannel-e2e-k8s-apiserver", node1YAML,
				kubectl("flannel-e2e-k8s-apiserver", "create", "-f", "-")...)
			return err
		}, 2*time.Minute, time.Second).Should(Succeed())

		By("creating Node flannel2")
		_, err = dockerExecI("flannel-e2e-k8s-apiserver", node2YAML,
			kubectl("flannel-e2e-k8s-apiserver", "create", "-f", "-")...)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		dockerRm("flannel-e2e-test-etcd")
		dockerRm("flannel-e2e-k8s-apiserver")
		dockerRm("flannel-e2e-test-flannel1")
		dockerRm("flannel-e2e-test-flannel2")
		if pkiDir != "" {
			Expect(os.RemoveAll(pkiDir)).To(Succeed())
		}
	})

	AfterEach(func() {
		dockerRm("flannel-e2e-test-flannel1")
		dockerRm("flannel-e2e-test-flannel2")
	})

	// startFlannel starts two kube-subnet-mgr flanneld containers for the given
	// backend, waits for /run/flannel/subnet.env, and returns ping destinations.
	startFlannel := func(backend string) (string, string) {
		GinkgoHelper()
		flannel_conf := fmt.Sprintf(`{ "Network": "%s", "Backend": { "Type": "%s" } }`, flannelNet, backend)

		dir, err := os.MkdirTemp("", "flannel-kube-")
		Expect(err).NotTo(HaveOccurred())

		out, err := dockerExec("flannel-e2e-k8s-apiserver", false,
			"cat", "/var/lib/kubernetes/admin.kubeconfig")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(dir, "admin.kubeconfig"), []byte(out), 0600)).To(Succeed())

		for _, num := range []string{"1", "2"} {
			name := "flannel-e2e-test-flannel" + num
			dockerRm(name)
			_, err := dockerRun(
				"-id", "--privileged",
				"-v", dir+":/var/lib/kubernetes/",
				"-e", "NODE_NAME=flannel"+num,
				"--name="+name,
				"--entrypoint", "/bin/sh",
				flannelDockerImage,
				"-c", fmt.Sprintf(
					`mkdir -p /etc/kube-flannel && echo '%s' > /etc/kube-flannel/net-conf.json && /opt/bin/flanneld --kube-subnet-mgr --ip-masq --kubeconfig-file /var/lib/kubernetes/admin.kubeconfig --kube-api-url %s`,
					flannel_conf, k8sEndpt,
				),
			)
			Expect(err).NotTo(HaveOccurred(), "starting flannel container %s", name)

			// Wait for subnet.env or container exit.
			err = waitForSubnetEnvOrExit(name)
			if err != nil {
				logf("flannel container %s logs:\n%s", name, dockerLogs(name))
			}
			Expect(err).NotTo(HaveOccurred())
		}

		pingDest1, err := createPingDest("flannel-e2e-test-flannel1")
		Expect(err).NotTo(HaveOccurred())
		pingDest2, err := createPingDest("flannel-e2e-test-flannel2")
		Expect(err).NotTo(HaveOccurred())
		return pingDest1, pingDest2
	}

	It("public-ip-overwrite annotation propagates", func() {
		// Annotate flannel1 with a static public IP.
		_, err := dockerExec("flannel-e2e-k8s-apiserver", false,
			kubectl("flannel-e2e-k8s-apiserver",
				"annotate", "node", "flannel1",
				"flannel.alpha.coreos.com/public-ip-overwrite=172.18.0.2",
			)...)
		Expect(err).NotTo(HaveOccurred())

		startFlannel("vxlan")

		By("checking public-ip annotation on flannel1")
		Eventually(func() string {
			out, _ := dockerExec("flannel-e2e-k8s-apiserver", false,
				kubectl("flannel-e2e-k8s-apiserver",
					"get", "node/flannel1", "-o",
					`jsonpath={.metadata.annotations.flannel\.alpha\.coreos\.com/public-ip}`,
				)...)
			return strings.TrimSpace(out)
		}, time.Minute, 2*time.Second).Should(Equal("172.18.0.2"),
			"overwriting public IP via annotation does not work")

		// Remove the annotation so later tests aren't affected.
		_, err = dockerExec("flannel-e2e-k8s-apiserver", false,
			kubectl("flannel-e2e-k8s-apiserver",
				"annotate", "node", "flannel1",
				"flannel.alpha.coreos.com/public-ip-overwrite-",
			)...)
		Expect(err).NotTo(HaveOccurred())
	})

	It("kube-flannel manifest is accepted by the API server", func() {
		dir, err := os.MkdirTemp("", "flannel-manifest-")
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(os.RemoveAll(dir)).To(Succeed())
		}()

		out, err := dockerExec("flannel-e2e-k8s-apiserver", false,
			"cat", "/var/lib/kubernetes/admin.kubeconfig")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(dir, "admin.kubeconfig"), []byte(out), 0600)).To(Succeed())

		manifest, err := os.ReadFile(k8sManifestPath)
		Expect(err).NotTo(HaveOccurred())

		_, err = dockerExecI("flannel-e2e-k8s-apiserver", string(manifest),
			kubectl("flannel-e2e-k8s-apiserver", "create", "-f", "-")...)
		Expect(err).NotTo(HaveOccurred(), "applying kube-flannel manifest")
	})
})

// kubectl returns a `kubectl --kubeconfig=... <args>` slice for use with
// dockerExec/dockerExecI against the apiserver container.
func kubectl(container string, args ...string) []string {
	base := append(strings.Fields(hyperkubeCmd), "kubectl", "--kubeconfig=/var/lib/kubernetes/admin.kubeconfig")
	return append(base, args...)
}

// waitForSubnetEnvOrExit waits for /run/flannel/subnet.env in the container,
// returning an error if the container exits before the file appears.
func waitForSubnetEnvOrExit(container string) error {
	for {
		_, err := runCommand("docker", "exec", container, "ls", "/run/flannel/subnet.env")
		if err == nil {
			return nil
		}
		status := dockerInspectStatus(container)
		if status != "running" {
			return fmt.Errorf("container %s exited with status %q before subnet.env appeared", container, status)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// generateKubePKI creates a temp directory with the PKI certs needed by the
// kube-apiserver container.  Returns the directory path.
func generateKubePKI(dockerIP string) (string, error) {
	dir, err := os.MkdirTemp("", "flannel-kube-pki-")
	if err != nil {
		return "", err
	}
	pkiDir := filepath.Join(dir, "pki")
	if err := os.MkdirAll(pkiDir, 0755); err != nil {
		return dir, err
	}

	// CA
	if _, err := runCommand("openssl", "genrsa", "-out", filepath.Join(pkiDir, "ca.key"), "2048"); err != nil {
		return dir, fmt.Errorf("gen ca.key: %w", err)
	}
	if _, err := runCommand("openssl", "req", "-new", "-key", filepath.Join(pkiDir, "ca.key"),
		"-subj", "/CN=KUBERNETES-CA/O=Kubernetes",
		"-out", filepath.Join(pkiDir, "ca.csr")); err != nil {
		return dir, fmt.Errorf("gen ca.csr: %w", err)
	}
	if _, err := runCommand("openssl", "x509", "-req",
		"-in", filepath.Join(pkiDir, "ca.csr"),
		"-signkey", filepath.Join(pkiDir, "ca.key"),
		"-CAcreateserial",
		"-out", filepath.Join(pkiDir, "ca.crt"),
		"-days", "1000"); err != nil {
		return dir, fmt.Errorf("gen ca.crt: %w", err)
	}

	// openssl.cnf with SAN
	cnfPath := filepath.Join(dir, "openssl.cnf")
	tmpl, err := template.New("cnf").Parse(opensslCNF)
	if err != nil {
		return dir, err
	}
	f, err := os.Create(cnfPath)
	if err != nil {
		return dir, err
	}
	if err := tmpl.Execute(f, struct{ DockerIP string }{dockerIP}); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return dir, fmt.Errorf("close %s after template execution failure: %v (template error: %w)", cnfPath, closeErr, err)
		}
		return dir, err
	}
	if err := f.Close(); err != nil {
		return dir, err
	}

	// kube-apiserver cert
	if _, err := runCommand("openssl", "genrsa", "-out", filepath.Join(pkiDir, "kube-apiserver.key"), "2048"); err != nil {
		return dir, fmt.Errorf("gen kube-apiserver.key: %w", err)
	}
	if _, err := runCommand("openssl", "req", "-new",
		"-key", filepath.Join(pkiDir, "kube-apiserver.key"),
		"-subj", "/CN=kube-apiserver/O=Kubernetes",
		"-out", filepath.Join(pkiDir, "kube-apiserver.csr"),
		"-config", cnfPath); err != nil {
		return dir, fmt.Errorf("gen kube-apiserver.csr: %w", err)
	}
	if _, err := runCommand("openssl", "x509", "-req",
		"-in", filepath.Join(pkiDir, "kube-apiserver.csr"),
		"-CA", filepath.Join(pkiDir, "ca.crt"),
		"-CAkey", filepath.Join(pkiDir, "ca.key"),
		"-CAcreateserial",
		"-out", filepath.Join(pkiDir, "kube-apiserver.crt"),
		"-extensions", "v3_req",
		"-extfile", cnfPath,
		"-days", "1000"); err != nil {
		return dir, fmt.Errorf("gen kube-apiserver.crt: %w", err)
	}

	// service-account key pair
	if _, err := runCommand("openssl", "genrsa", "-out", filepath.Join(pkiDir, "service-account.key"), "2048"); err != nil {
		return dir, fmt.Errorf("gen service-account.key: %w", err)
	}
	if _, err := runCommand("openssl", "req", "-new",
		"-key", filepath.Join(pkiDir, "service-account.key"),
		"-subj", "/CN=service-accounts/O=Kubernetes",
		"-out", filepath.Join(pkiDir, "service-account.csr")); err != nil {
		return dir, fmt.Errorf("gen service-account.csr: %w", err)
	}
	if _, err := runCommand("openssl", "x509", "-req",
		"-in", filepath.Join(pkiDir, "service-account.csr"),
		"-CA", filepath.Join(pkiDir, "ca.crt"),
		"-CAkey", filepath.Join(pkiDir, "ca.key"),
		"-CAcreateserial",
		"-out", filepath.Join(pkiDir, "service-account.crt"),
		"-days", "100"); err != nil {
		return dir, fmt.Errorf("gen service-account.crt: %w", err)
	}

	// admin cert
	if _, err := runCommand("openssl", "genrsa", "-out", filepath.Join(pkiDir, "admin.key"), "2048"); err != nil {
		return dir, fmt.Errorf("gen admin.key: %w", err)
	}
	if _, err := runCommand("openssl", "req", "-new",
		"-key", filepath.Join(pkiDir, "admin.key"),
		"-subj", "/CN=admin/O=system:masters",
		"-out", filepath.Join(pkiDir, "admin.csr")); err != nil {
		return dir, fmt.Errorf("gen admin.csr: %w", err)
	}
	if _, err := runCommand("openssl", "x509", "-req",
		"-in", filepath.Join(pkiDir, "admin.csr"),
		"-CA", filepath.Join(pkiDir, "ca.crt"),
		"-CAkey", filepath.Join(pkiDir, "ca.key"),
		"-CAcreateserial",
		"-out", filepath.Join(pkiDir, "admin.crt"),
		"-days", "1000"); err != nil {
		return dir, fmt.Errorf("gen admin.crt: %w", err)
	}

	return dir, nil
}

// setupKubeconfig runs the `kubectl config set-*` commands inside the apiserver
// container to produce /var/lib/kubernetes/admin.kubeconfig.
func setupKubeconfig(dockerIP string) error {
	cmds := [][]string{
		{"config", "set-cluster", "kubernetes-test-flannel",
			"--certificate-authority=/var/lib/kubernetes/pki/ca.crt",
			"--embed-certs=true",
			"--server=https://" + dockerIP + ":6443",
			"--kubeconfig=/var/lib/kubernetes/admin.kubeconfig"},
		{"config", "set-credentials", "admin",
			"--client-certificate=/var/lib/kubernetes/pki/admin.crt",
			"--client-key=/var/lib/kubernetes/pki/admin.key",
			"--embed-certs=true",
			"--kubeconfig=/var/lib/kubernetes/admin.kubeconfig"},
		{"config", "set-context", "default",
			"--cluster=kubernetes-test-flannel",
			"--user=admin",
			"--kubeconfig=/var/lib/kubernetes/admin.kubeconfig"},
		{"config", "use-context", "default",
			"--kubeconfig=/var/lib/kubernetes/admin.kubeconfig"},
	}
	for _, args := range cmds {
		cmd := append(append([]string{}, strings.Fields(hyperkubeCmd)...), "kubectl")
		cmd = append(cmd, args...)
		out, err := dockerExec("flannel-e2e-k8s-apiserver", false, cmd...)
		if err != nil {
			return fmt.Errorf("kubectl %v: %w\n%s", args, err, out)
		}
	}
	return nil
}
