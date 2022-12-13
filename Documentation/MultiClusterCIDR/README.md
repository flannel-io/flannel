Flannel provides experimental support for the new [MultiClusterCIDR API](https://github.com/kubernetes/enhancements/tree/master/keps/sig-network/2593-multiple-cluster-cidrs) introduced as an alpha feature in Kubernetes 1.26.

## Prerequisites
* A cluster running Kubernetes 1.26 (this was tested on version `1.26.0-alpha.1`)
* Use flannel version `0.21.0` or later
* The MultiClusterCIDR API can be used with vxlan, wireguard and host-gw backend

*Note*: once a PodCIDR is allocated to a node, it cannot be modified or removed. So you need to configure the MultiClusterCIDR before you add the new nodes to your cluster.

## How to use the MultiClusterCIDR API
### Enable the new API in the control plane
* Edit `/etc/kubernetes/manifests/kube-controller-manager.yaml` and add the following lines in the `spec.containers.command` section:
```
    - --cidr-allocator-type=MultiCIDRRangeAllocator
    - --feature-gates=MultiCIDRRangeAllocator=true
```

* Edit `/etc/kubernetes/manifests/kube-apiserver.yaml` and add the following line in the `spec.containers.command` section:
```
    - --runtime-config=networking.k8s.io/v1alpha1
```

Both components should restart automatically and a default ClusterCIDR resource will be created based on the usual `pod-network-cidr` parameter.

For example:
```bash
$ kubectl  get clustercidr
NAME                   PERNODEHOSTBITS   IPV4            IPV6                 AGE
default-cluster-cidr   8                 10.244.0.0/16   2001:cafe:42::/112   24h

$ kubectl describe clustercidr default-cluster-cidr
Name:         default-cluster-cidr
Labels:       <none>
Annotations:  <none>
NodeSelector:
PerNodeHostBits:  8
IPv4:             10.244.0.0/16
IPv6:             2001:cafe:42::/112
Events:           <none>
```

### Enable the new feature in flannel
This feature is disabled by default. To enable it, add the following flag to the args of the `kube-flannel` container:
```
    - --use-multi-cluster-cidr
```

Since you will specify the subnets to use for pods IP addresses through the new API, you do not need the `Network` and `IPv6Network` sections in the flannel configuration. Thus your flannel configuration could look like this:
```json
{
    "EnableIPv6": true,
    "Backend": {
    "Type": "host-gw"
    }
}
```


If you let them in, they will simply be ignored by flannel.
NOTE: this only applies when using the MultiClusterCIDR API.

### Configure the required `clustercidr` resources
Before adding nodes to the cluster, you need to add new `clustercidr` resources.

For example:
```yaml
apiVersion: networking.k8s.io/v1alpha1
kind: ClusterCIDR
metadata:
  name: my-cidr-1
spec:
  nodeSelector:
    nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          -  "worker1"
  perNodeHostBits:      8
  ipv4: 10.248.0.0/16
  ipv6: 2001:cafe:43::/112
---
apiVersion: networking.k8s.io/v1alpha1
kind: ClusterCIDR
metadata:
  name: my-cidr-2
spec:
  nodeSelector:
    nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          -  "worker2"
  perNodeHostBits:      8
  ipv4: 10.247.0.0/16
  ipv6: ""
```
For more details on the `spec` section, see the [feature specification page](https://github.com/kubernetes/enhancements/tree/master/keps/sig-network/2593-multiple-cluster-cidrs#expected-behavior).

*WARNING*: all the fields in the `spec` section are immutable.

For more information on Node Selectors, see [the Kubernetes documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/).

### Add nodes to the cluster
The new nodes will be allocated a `PodCIDR` based on the configured `clustercidr`.
flannel will ensure connectivity between all the pods regardless of the subnet in which the pod's IP address has been allocated.

## Notes on the subnet.env file
flanneld writes a file (located by default at /run/flannel/subnet.env) that is used by the flannel cni plugin which is called by the kubelet every time a pod is added or removed from the node. This file changes slightly with the new API. The `FLANNEL_NETWORK` and `FLANNEL_IPV6_NETWORK` become lists of CIDRs instead of sigle CIDR entry. They will hold the list of CIDRs declared in the `clustercidr` resource of the API. The file is updated by flanneld every time a new `clustercidr` is created.

As an example, it could look like this:
```bash
FLANNEL_NETWORK=10.42.0.0/16,192.168.0.0/16
FLANNEL_SUBNET=10.42.0.1/24
FLANNEL_IPV6_NETWORK=2001:cafe:42::/56
FLANNEL_IPV6_SUBNET=2001:cafe:42::1/64,2001:cafd:42::1/64
FLANNEL_MTU=1450
FLANNEL_IPMASQ=true
```

## Notes on using IPv6 with the MultiClusterCIDR API
The feature is fully compatible with IPv6 and dual-stack networking.
Each `clustercidr` resource can include an IPv4 and/or an IPv6 subnet.
If both are provided, the PodCIDR allocated based on this `clustercidr` will be dual-stack.
The controller allows you to use IPv4, IPv6 and dual-stack `clustercidr` resources all at the same time to facilitate cluster migrations.
As a result, it is up to you to ensure the coherence of your IP allocation.

If you want to use dual-stack networking with the new API, we recommend that you do not specify the `--pod-network-cidr` flag to `kubeadm` when installing the cluster so that you can manually configure the controller later.
In that case, when you edit `/etc/kubernetes/manifests/kube-controller-manager.yaml`, add:
```
    - --cidr-allocator-type=MultiCIDRRangeAllocator
    - --feature-gates=MultiCIDRRangeAllocator=true
    - --cluster-cidr=10.244.0.0/16,2001:cafe:42::/112 #replace with your own default clusterCIDR
    - --node-cidr-mask-size-ipv6=120
    - --allocate-node-cidrs
```
