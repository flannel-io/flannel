# flannel

![flannel Logo](logos/flannel-horizontal-color.png)

[![Build Status](https://travis-ci.org/coreos/flannel.png?branch=master)](https://travis-ci.org/coreos/flannel)

Flannel is a virtual network that gives a subnet to each host for use with container runtimes.

Platforms like Kubernetes assume that each container (pod) has a unique, routable IP inside the cluster. The advantage of this model is that it reduces the complexity of doing port mapping.

## How it works

Flannel runs an agent, `flanneld`, on each host and is responsible for allocating a subnet lease out of a preconfigured address space. Flannel uses either [etcd][etcd] or the Kubernetes API to store the network configuration, allocated subnets, and auxiliary data (such as host's IP). Packets are forwarded using one of several [backend mechanisms][backends].

The following diagram demonstrates the path a packet takes as it traverses the overlay network:

![Life of a packet](./packet-01.png)

## Getting started

The easiest way to deploy flannel with Kubernetes is to use one of several deployment tools and distributions that network clusters with flannel by default. CoreOS's [Tectonic][tectonic] sets up flannel in the Kubernetes clusters it creates using the open source [Tectonic Installer][tectonic-installer] to drive the setup process.

Flannel can use the Kubernetes API as its backing store, meaning there's no need to deploy a discrete `etcd` cluster for `flannel`. This `flannel` mode is known as the *kube subnet manager*.

### Adding flannel

Flannel can be added to any existing Kubernetes cluster. It's simplest to add `flannel` before any pods using the pod network have been started.

For information on deploying flannel manually, using the (currently alpha) Kubernetes installer toolkit kubeadm, see [Installing Kubernetes on Linux with kubeadm][installing-with-kubeadm].

### Using flannel

Once applied, the `flannel` manifest defines three things:
1. A service account for `flannel` to use.
2. A ConfigMap containing both a CNI configuration and a `flannel` configuration. The network in the `flannel` configuration should match the pod network CIDR. The choice of `backend` is also made here and defaults to VXLAN.
3. A DaemonSet to deploy the `flannel` pod on each Node. The pod has two containers 1) the `flannel` daemon itself, and 2) a container for deploying the CNI configuration to a location that the `kubelet` can read.

When you run pods, they will be allocated IP addresses from the pod network CIDR. No matter which node those pods end up on, they will be able to communicate with each other.

Kubernetes 1.6 requires CNI plugin version 0.5.1 or later.

## Documentation
- [Building (and releasing)](Documentation/building.md)
- [Configuration](Documentation/configuration.md)
- [Backends](Documentation/backends.md)
- [Running](Documentation/running.md)
- [Troubleshooting](Documentation/troubleshooting.md)
- [Projects integrating with flannel](Documentation/integrations.md)
- [Production users](Documentation/production-users.md)

## Contact

* Mailing list: coreos-dev
* IRC: #coreos on freenode.org
* Planning/Roadmap: [milestones][milestones], [roadmap][roadmap]
* Bugs: [issues][flannel-issues]

## Contributing

See [CONTRIBUTING][contributing] for details on submitting patches and the contribution workflow.

## Reporting bugs

See [reporting bugs][reporting] for details about reporting any issues.

## License

Flannel is under the Apache 2.0 license. See the [LICENSE][license] file for details.


[kubeadm]: https://kubernetes.io/docs/getting-started-guides/kubeadm/
[pod-cidr]: https://kubernetes.io/docs/admin/kubelet/
[etcd]: https://github.com/coreos/etcd
[contributing]: CONTRIBUTING.md
[license]: https://github.com/coreos/flannel/blob/master/LICENSE
[milestones]: https://github.com/coreos/flannel/milestones
[flannel-issues]: https://github.com/coreos/flannel/issues
[backends]: Documentation/backends.md
[roadmap]: https://github.com/kubernetes/kubernetes/milestones
[reporting]: Documentation/reporting_bugs.md
[tectonic-installer]: https://github.com/coreos/tectonic-installer
[installing-with-kubeadm]: https://kubernetes.io/docs/getting-started-guides/kubeadm/
[tectonic]: https://coreos.com/tectonic/
