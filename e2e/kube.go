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
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// createTestPod creates a simple multitool pod pinned to a node, mirroring
// create_test_pod.
func (kc *kindCluster) createTestPod(ctx context.Context, name, node string) error {
	return kc.createPinnedPod(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PodSpec{
			NodeName: node,
			Containers: []corev1.Container{{
				Name:  name,
				Image: multitoolImage,
			}},
		},
	})
}

// createIperfServerPod mirrors create_iperf_server_pod.
func (kc *kindCluster) createIperfServerPod(ctx context.Context, name, node string) error {
	return kc.createPinnedPod(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PodSpec{
			NodeName: node,
			Containers: []corev1.Container{{
				Name:            name,
				Image:           iperf3Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
			}},
		},
	})
}

// createIperfClientPod mirrors create_iperf_client_pod (kept alive via sleep).
func (kc *kindCluster) createIperfClientPod(ctx context.Context, name, node string) error {
	return kc.createPinnedPod(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PodSpec{
			NodeName: node,
			Containers: []corev1.Container{{
				Name:            name,
				Image:           iperf3Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/bin/sh", "-c", "while true; do sleep 3600; done"},
			}},
		},
	})
}

func (kc *kindCluster) createPinnedPod(ctx context.Context, pod *corev1.Pod) error {
	_, err := kc.client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	return err
}

// deleteTestPods removes the well-known test pods left over from a prior spec.
// Called at the start of prepareTest so that the shared cluster starts clean
// for each backend, matching the bash suite's single-cluster approach.
func (kc *kindCluster) deleteTestPods(ctx context.Context) error {
	testPods := []string{"multitool1", "multitool2", "iperf3-server", "iperf3-client"}
	for _, name := range testPods {
		gracePeriodSeconds := int64(0)
		err := kc.client.CoreV1().Pods("default").Delete(ctx, name, metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriodSeconds,
		})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting pod %s: %w", name, err)
		}
	}
	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, time.Minute, true, func(ctx context.Context) (bool, error) {
		for _, name := range testPods {
			_, err := kc.client.CoreV1().Pods("default").Get(ctx, name, metav1.GetOptions{})
			switch {
			case errors.IsNotFound(err):
				continue
			case err != nil:
				return false, err
			default:
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("waiting for test pod deletion: %w", err)
	}
	return nil
}

// podIP mirrors get_pod_ip.
func (kc *kindCluster) podIP(ctx context.Context, name string) (string, error) {
	pod, err := kc.client.CoreV1().Pods("default").Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return pod.Status.PodIP, nil
}

// podCIDR mirrors get_pod_cidr.
func (kc *kindCluster) podCIDR(ctx context.Context, node string) (string, error) {
	n, err := kc.client.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return n.Spec.PodCIDR, nil
}

// podCIDRv6 returns the node's IPv6 pod CIDR from Spec.PodCIDRs (dual-stack).
func (kc *kindCluster) podCIDRv6(ctx context.Context, node string) (string, error) {
	n, err := kc.client.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, cidr := range n.Spec.PodCIDRs {
		if strings.Contains(cidr, ":") {
			return cidr, nil
		}
	}
	return "", fmt.Errorf("node %s has no IPv6 pod CIDR (PodCIDRs=%v)", node, n.Spec.PodCIDRs)
}

// podIPv6 returns the pod's IPv6 address from Status.PodIPs (dual-stack).
func (kc *kindCluster) podIPv6(ctx context.Context, name string) (string, error) {
	pod, err := kc.client.CoreV1().Pods("default").Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, podIP := range pod.Status.PodIPs {
		if strings.Contains(podIP.IP, ":") {
			return podIP.IP, nil
		}
	}
	return "", fmt.Errorf("pod %s has no IPv6 address (PodIPs=%v)", name, pod.Status.PodIPs)
}

// execInPod runs a command in a pod's default container and returns
// stdout+stderr, replacing `kubectl exec`.
func (kc *kindCluster) execInPod(ctx context.Context, namespace, pod string, command ...string) (string, error) {
	req := kc.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: command,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(kc.restCfg, "POST", req.URL())
	if err != nil {
		return "", err
	}

	var stdout, stderr bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	out := stdout.String() + stderr.String()
	return out, err
}

// waitForPodReady polls until the named pod's containers are all ready.
func (kc *kindCluster) waitForPodReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 3*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := kc.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if len(pod.Status.ContainerStatuses) == 0 {
			return false, nil
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				return false, nil
			}
		}
		return true, nil
	})
}

// waitForNodesReady mirrors e2e-wait-for-nodes (>= 2 Ready nodes).
func (kc *kindCluster) waitForNodesReady(ctx context.Context, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		nodeList, err := kc.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		ready := 0
		for _, n := range nodeList.Items {
			for _, c := range n.Status.Conditions {
				if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
					ready++
				}
			}
		}
		return ready >= 2, nil
	})
}

// waitForCoreDNS mirrors e2e-wait-for-services for coredns.
func (kc *kindCluster) waitForCoreDNS(ctx context.Context, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := kc.client.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: "k8s-app=kube-dns",
		})
		if err != nil || len(pods.Items) == 0 {
			return false, nil
		}
		for _, p := range pods.Items {
			for _, cs := range p.Status.ContainerStatuses {
				if !cs.Ready {
					return false, nil
				}
			}
		}
		return true, nil
	})
}

// waitForPing mirrors e2e-wait-for-ping: retry a single ping until it succeeds.
func (kc *kindCluster) waitForPing(ctx context.Context, pod, ip string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := kc.execInPod(ctx, "default", pod, "ping", "-c", "1", ip)
		return err == nil, nil
	})
}

// waitForPing6 is the IPv6 counterpart of waitForPing.
func (kc *kindCluster) waitForPing6(ctx context.Context, pod, ip string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := kc.execInPod(ctx, "default", pod, "ping", "-6", "-c", "1", ip)
		return err == nil, nil
	})
}

// waitForSubnetEnv mirrors wait_for_subnet_env: ensure /run/flannel/subnet.env
// exists on every node (checked by exec-ing into the kind node containers).
func (kc *kindCluster) waitForSubnetEnv(ctx context.Context, timeout time.Duration) error {
	for _, node := range []string{kindControlPlane, kindWorker} {
		err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
			_, err := kc.execOnNode(node, "test", "-f", "/run/flannel/subnet.env")
			return err == nil, nil
		})
		if err != nil {
			return fmt.Errorf("timed out waiting for /run/flannel/subnet.env on %s: %w", node, err)
		}
		out, _ := kc.execOnNode(node, "cat", "/run/flannel/subnet.env")
		logf("subnet.env on %s:\n%s\n", node, out)
	}
	return nil
}

// createDeployment creates a Deployment in the default namespace serving on the
// given container port. Replicas are guaranteed to spread across nodes via a
// required pod anti-affinity rule so that pod-to-service traffic must cross the
// flannel overlay. The control-plane NoSchedule taint is tolerated so a replica
// can run there in the default kind cluster. When node is non-empty the pods are
// additionally pinned to that node.
func (kc *kindCluster) createDeployment(ctx context.Context, name, image string, replicas int32, port int32, node string) error {
	labels := map[string]string{"app": name}
	spec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:            name,
			Image:           image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports:           []corev1.ContainerPort{{ContainerPort: port}},
		}},
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
					TopologyKey:   "kubernetes.io/hostname",
				}},
			},
		},
		Tolerations: []corev1.Toleration{{
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		}},
	}
	if node != "" {
		spec.NodeName = node
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       spec,
			},
		},
	}
	_, err := kc.client.AppsV1().Deployments("default").Create(ctx, dep, metav1.CreateOptions{})
	return err
}

// createClusterIPService creates a ClusterIP Service in the default namespace
// selecting pods by the given selector.
func (kc *kindCluster) createClusterIPService(ctx context.Context, name string, selector map[string]string, port, targetPort int32) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Port:       port,
				TargetPort: intstr.FromInt32(targetPort),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
	_, err := kc.client.CoreV1().Services("default").Create(ctx, svc, metav1.CreateOptions{})
	return err
}

// serviceClusterIP returns the allocated ClusterIP of a Service.
func (kc *kindCluster) serviceClusterIP(ctx context.Context, namespace, name string) (string, error) {
	svc, err := kc.client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return svc.Spec.ClusterIP, nil
}

// waitForDeploymentReady polls until all replicas of the Deployment are ready.
func (kc *kindCluster) waitForDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 3*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		dep, err := kc.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		desired := int32(1)
		if dep.Spec.Replicas != nil {
			desired = *dep.Spec.Replicas
		}
		return dep.Status.ReadyReplicas >= desired, nil
	})
}

// deleteDeployment removes a Deployment, ignoring NotFound.
func (kc *kindCluster) deleteDeployment(ctx context.Context, namespace, name string) error {
	err := kc.client.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting deployment %s: %w", name, err)
	}
	return nil
}

// deleteService removes a Service, ignoring NotFound.
func (kc *kindCluster) deleteService(ctx context.Context, namespace, name string) error {
	err := kc.client.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting service %s: %w", name, err)
	}
	return nil
}

// deletePod removes a single pod immediately, ignoring NotFound.
func (kc *kindCluster) deletePod(ctx context.Context, namespace, name string) error {
	gracePeriodSeconds := int64(0)
	err := kc.client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
	})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting pod %s: %w", name, err)
	}
	return nil
}
