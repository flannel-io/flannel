# flannel

![flannel Logo](logos/flannel-horizontal-color.png)

[![Build Status](https://travis-ci.org/coreos/flannel.png?branch=master)](https://travis-ci.org/coreos/flannel)

Flannel is a simple and easy way to configure a layer 3 network fabric designed for Kubernetes.

## How it works

Flannel runs a small, single binary agent called `flanneld` on each host, and is responsible for allocating a subnet lease to each host out of a larger, preconfigured address space.
Flannel uses either the Kubernetes API or [etcd][etcd] directly to store the network configuration, the allocated subnets, and any auxiliary data (such as the host's public IP).
Packets are forwarded using one of several [backend mechanisms][backends] including VXLAN and various cloud integrations.

### Networking details

Platforms like Kubernetes assume that each container (pod) has a unique, routable IP inside the cluster.
The advantage of this model is that it removes the port mapping complexities that come from sharing a single host IP.

Flannel is responsible for providing a layer 3 IPv4 network between multiple nodes in a cluster. Flannel does not control how containers are networked to the host, only how the traffic is transported between hosts. However, flannel does provide a CNI plugin for Kubernetes and a guidance on integrating with Docker.

Flannel is focused on networking. For network policy, other projects such as [Calico][calico] can be used.

## Getting started on Kubernetes

The easiest way to deploy flannel with Kubernetes is to use one of several deployment tools and distributions that network clusters with flannel by default. For example, [K3s][k3s] sets up flannel in the Kubernetes clusters it creates using the open source [K3s Installer][k3s-installer] to drive the setup process.

Though not required, it's recommended that flannel uses the Kubernetes API as its backing store which avoids the need to deploy a discrete `etcd` cluster for `flannel`. This `flannel` mode is known as the *kube subnet manager*.

### Deploying flannel manually

Flannel can be added to any existing Kubernetes cluster though it's simplest to add `flannel` before any pods using the pod network have been started.

For Kubernetes v1.17+
```
kubectl apply -f https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml
```

If you use custom `podCIDR` (not `10.244.0.0/16`) you first need to download the above manifest and modify the network to match your one.

See [Kubernetes](Documentation/kubernetes.md) for more details.

## Getting started on Docker

flannel is also widely used outside of kubernetes. When deployed outside of kubernetes, etcd is always used as the datastore. For more details integrating flannel with Docker see [Running](Documentation/running.md)

## Documentation
- [Building (and releasing)](Documentation/building.md)
- [Configuration](Documentation/configuration.md)
- [Backends](Documentation/backends.md)
- [Running](Documentation/running.md)
- [Troubleshooting](Documentation/troubleshooting.md)
- [Projects integrating with flannel](Documentation/integrations.md)
- [Production users](Documentation/production-users.md)

## Contact

* Slack: 
   * #k3s on [Rancher Users Slack](https://slack.rancher.io)
   * #flannel-users on [Calico Users Slack](https://slack.projectcalico.org)
* Planning/Roadmap: [milestones][milestones], [roadmap][roadmap]
* Bugs: [issues][flannel-issues]

## Community Meeting

The Flannel Maintainer Community runs a meeting on every other Thursday at 8:30 AM PST. This meeting is used to discuss issues, open pull requests, and other topics related to Flannel should the need arise.

The meeting agenda and Teams link can be found here: [Flannel Community Meeting Agenda](https://docs.google.com/document/d/1kPMMFDhljWL8_CUZajrfL8Q9sdntd9vvUpe-UGhX5z8)

## Contributing

See [CONTRIBUTING][contributing] for details on submitting patches and the contribution workflow.

## Reporting bugs

See [reporting bugs][reporting] for details about reporting any issues.

## Licensing

Flannel is under the Apache 2.0 license. See the [LICENSE][license] file for details.

[calico]: http://www.projectcalico.org
[pod-cidr]: https://kubernetes.io/docs/admin/kubelet/
[etcd]: https://go.etcd.io/etcd/v3
[contributing]: CONTRIBUTING.md
[license]: https://github.com/flannel-io/flannel/blob/master/LICENSE
[milestones]: https://github.com/flannel-io/flannel/milestones
[flannel-issues]: https://github.com/flannel-io/flannel/issues
[backends]: Documentation/backends.md
[roadmap]: https://github.com/kubernetes/kubernetes/milestones
[reporting]: Documentation/reporting_bugs.md
[k3s-installer]: https://github.com/k3s-io/k3s/#quick-start---install-script
[installing-with-kubeadm]: https://kubernetes.io/docs/getting-started-guides/kubeadm/
[k3s]: https://k3s.io/
