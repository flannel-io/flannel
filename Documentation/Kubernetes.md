# kubeadm

For information on deploying flannel manually, using the Kubernetes installer toolkit kubeadm, see [Installing Kubernetes on Linux with kubeadm][kubeadm].

NOTE: If `kubeadm` is used, then pass `--pod-network-cidr=10.244.0.0/16` to `kubeadm init` to ensure that the `podCIDR` is set.

kubeadm has RBAC enabled by default so you must apply the `kube-flannel-rbac.yml` manifest as well as the `kube-flannel.yml` manifest.

* `kubectl apply -f kube-flannel-rbac.yml -f kube-flannel.yml`

If you didn't apply the `kube-flannel-rbac.yml` manifest, you'll see errors in your flanneld logs about failing to connect. 
* `Failed to create SubnetManager: error retrieving pod spec...`

If you forgot to apply the `kube-flannel-rbac.yml` manifest and notice that flannel fails to start, then it is safe to just apply the `kube-flannel-rbac.yml` manifest without running `kubectl delete -f kube-flannel.yaml` first.
* `kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel-rbac.yml`

# kube-flannel.yaml

The `flannel` manifest defines three things:
1. A service account for `flannel` to use.
2. A ConfigMap containing both a CNI configuration and a `flannel` configuration. The `network` in the `flannel` configuration should match the pod network CIDR. The choice of `backend` is also made here and defaults to VXLAN.
3. A DaemonSet to deploy the `flannel` pod on each Node. The pod has two containers 1) the `flannel` daemon itself, and 2) a container for deploying the CNI configuration to a location that the `kubelet` can read.

When you run pods, they will be allocated IP addresses from the pod network CIDR. No matter which node those pods end up on, they will be able to communicate with each other.

## The flannel CNI plugin

The flannel CNI plugin can be found in the CNI plugins [reposistory](https://github.com/containernetworking/plugins). For additional details, see the [README](https://github.com/containernetworking/plugins/tree/master/plugins/meta/flannel)

Kubernetes 1.6 requires CNI plugin version 0.5.1 or later.

# Troubleshooting

See [troubleshooting](troubleshooting.md)

[kubeadm]: https://kubernetes.io/docs/getting-started-guides/kubeadm/
