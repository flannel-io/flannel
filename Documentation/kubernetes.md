# kubeadm

For information on deploying flannel manually, using the Kubernetes installer toolkit kubeadm, see [Installing Kubernetes on Linux with kubeadm][kubeadm].

NOTE: If `kubeadm` is used, then pass `--pod-network-cidr=10.244.0.0/16` to `kubeadm init` to ensure that the `podCIDR` is set.

# kube-flannel.yaml

The `flannel` manifest defines four things:
1. A ClusterRole and ClusterRoleBinding for role based acccess control (RBAC).
2. A service account for `flannel` to use.
3. A ConfigMap containing both a CNI configuration and a `flannel` configuration. The `network` in the `flannel` configuration should match the pod network CIDR. The choice of `backend` is also made here and defaults to VXLAN.
4. A DaemonSet for every architecture to deploy the `flannel` pod on each Node. The pod has two containers 1) the `flannel` daemon itself, and 2) an initContainer for deploying the CNI configuration to a location that the `kubelet` can read.

When you run pods, they will be allocated IP addresses from the pod network CIDR. No matter which node those pods end up on, they will be able to communicate with each other.

# Annotations

*  `flannel.alpha.coreos.com/public-ip-overwrite`: Allows to overwrite the public IP of a node. Useful if the public IP can not determined from the node, e.G. because it is behind a NAT. It can be automatically set to a nodes `ExternalIP` using the [flannel-node-annotator](https://github.com/alvaroaleman/flannel-node-annotator)

## Older versions of Kubernetes

`kube-flannel.yaml` has some features that aren't compatible with older versions of Kubernetes, though flanneld itself should work with any version of Kubernetes.

If you see errors saying `found invalid field...` when you try to apply `kube-flannel.yaml` then you can try the "legacy" manifest file
* `kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/k8s-manifests/kube-flannel-legacy.yml`

This file does not bundle RBAC permissions. If you need those, run
* `kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/k8s-manifests/kube-flannel-rbac.yml`

If you didn't apply the `kube-flannel-rbac.yml` manifest and you need to, you'll see errors in your flanneld logs about failing to connect.
* `Failed to create SubnetManager: error retrieving pod spec...`

## The flannel CNI plugin

The flannel CNI plugin can be found in the CNI plugins [repository](https://github.com/containernetworking/plugins). For additional details, see the [README](https://github.com/containernetworking/plugins/tree/master/plugins/meta/flannel)

Kubernetes 1.6 requires CNI plugin version 0.5.1 or later.

# Troubleshooting

See [troubleshooting](troubleshooting.md)

[kubeadm]: https://kubernetes.io/docs/getting-started-guides/kubeadm/
