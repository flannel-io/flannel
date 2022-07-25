# kubeadm

For information on deploying flannel manually, using the Kubernetes installer toolkit kubeadm, see [Installing Kubernetes on Linux with kubeadm][kubeadm].

NOTE: If `kubeadm` is used, then pass `--pod-network-cidr=10.244.0.0/16` to `kubeadm init` to ensure that the `podCIDR` is set.

# kube-flannel.yaml

The `flannel` manifest defines five things:
1. A `kube-flannel` with PodSecurity level set to *privileged*. 
2. A ClusterRole and ClusterRoleBinding for Role Based Acccess Control (RBAC).
3. A service account for `flannel` to use.
4. A ConfigMap containing both a CNI configuration and a `flannel` configuration. The `network` in the `flannel` configuration should match the pod network CIDR. The choice of `backend` is also made here and defaults to VXLAN.
5. A DaemonSet for every architecture to deploy the `flannel` pod on each Node. The pod has two containers 1) the `flannel` daemon itself, and 2) an initContainer for deploying the CNI configuration to a location that the `kubelet` can read.

When you run pods, they will be allocated IP addresses from the pod network CIDR. No matter which node those pods end up on, they will be able to communicate with each other.

# Notes on securing flannel deployment
As of Kubernetes v1.21, the [PodSecurityPolicy API was deprecated](https://kubernetes.io/blog/2021/04/06/podsecuritypolicy-deprecation-past-present-and-future/) and it will be removed in v1.25. Thus, the `flannel` manifest does not use PodSecurityPolicy anymore. 

If you wish to use the [Pod Security Admission Controller](https://kubernetes.io/docs/concepts/security/pod-security-admission/) which was introduced to [replace PodSecurityPolicy](https://kubernetes.io/docs/tasks/configure-pod-container/migrate-from-psp/), you will need to deploy `flannel` in a namespace which allows the deployment of pods with `privileged` level. The `baseline` level is insufficient to deploy `flannel` and you will see the following error message:
```
Error creating: non-default capabilities (container "kube-flannel" must not include "NET_ADMIN", "NET_RAW" in securityContext.capabilities.add), host namespaces (hostNetwork=true), hostPath volumes (volumes "run", "cni-plugin", "cni", "xtables-lock")
```

The `kube-flannel.yaml` manifest deploys `flannel` in the `kube-flannel` namespace and enables the `privileged` level for this namespace. 
Thus, you will need to restrict access to this namespace if you wish to secure your cluster.

If you want to deploy `flannel` securely in a shared namespace or want more fine-grained control over the pods deployed in your cluster, you can use a 3rd-party admission controller like [Kubewarden](https://kubewarden.io). Kubewarden provides policies that can replace features of PodSecurityPolicy like [capabilities-psp-policy](https://github.com/kubewarden/capabilities-psp-policy) and [hostpaths-psp-policy](https://github.com/kubewarden/hostpaths-psp-policy).

Other options include [Kyverno](https://kyverno.io/policies/pod-security/) and [OPA Gatekeeper](https://github.com/open-policy-agent/gatekeeper).
# Annotations

*  `flannel.alpha.coreos.com/public-ip-overwrite`: Allows to overwrite the public IP of a node. Useful if the public IP can not determined from the node, e.G. because it is behind a NAT. It can be automatically set to a nodes `ExternalIP` using the [flannel-node-annotator](https://github.com/alvaroaleman/flannel-node-annotator)

## Older versions of Kubernetes

`kube-flannel.yaml` has some features that aren't compatible with older versions of Kubernetes, though flanneld itself should work with any version of Kubernetes.

### For Kubernetes v1.6~v1.15

If you see errors saying `found invalid field...` when you try to apply `kube-flannel.yaml` then you can try the "legacy" manifest file
* `kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/k8s-manifests/kube-flannel-legacy.yml`

This file does not bundle RBAC permissions. If you need those, run
* `kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/k8s-manifests/kube-flannel-rbac.yml`

If you didn't apply the `kube-flannel-rbac.yml` manifest and you need to, you'll see errors in your flanneld logs about failing to connect.
* `Failed to create SubnetManager: error retrieving pod spec...`

### For Kubernetes v1.16

`kube-flannel.yaml` uses `ClusterRole` & `ClusterRoleBinding` of `rbac.authorization.k8s.io/v1`. When you use Kubernetes v1.16, you should replace `rbac.authorization.k8s.io/v1` to `rbac.authorization.k8s.io/v1beta1` because `rbac.authorization.k8s.io/v1` had become GA from Kubernetes v1.17.

### For Kubernetes <= v1.24
As of Kubernetes v1.21, the [PodSecurityPolicy API was deprecated](https://kubernetes.io/blog/2021/04/06/podsecuritypolicy-deprecation-past-present-and-future/) and it will be removed in v1.25. Thus, the `flannel` manifest does not use PodSecurityPolicy anymore.

If you still wish to use it, you can use `kube-flannel-psp.yaml` instead of `kube-flannel.yaml`. Please note that if you use a Kubernetes version >= 1.21, you will see a deprecation warning for the PodSecurityPolicy API.

# Troubleshooting

See [troubleshooting](troubleshooting.md)

[kubeadm]: https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/
