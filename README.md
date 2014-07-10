# kolach

kolach is a point to point VPN that assigns a subnet to each machine for use with
k8s.

In k8s every machine in the cluster is assigned a full subnet. The machine A
and B might have 10.0.1.0/24 and 10.0.2.0/24 respectively. The advantage of
this model is that it reduces the complexity of doing port mapping. The
disadvantage is that the only cloud provider that can do this is GCE.

## Theory of Operation

To emulate the Kubernetes model from GCE on other platforms we need to create
an overlay network on top of the network that we are given from cloud
providers. Not a fun task but certainly doable.

There are few prototype steps we need to explore to bring this together:

1) Get openvpn (or some similar product) working inside of a container and
bridging a subnet between CoreOS machines.

This blog post outline a configuration that can probably work for openvpn:
http://blog.wains.be/2008/06/07/routed-openvpn-between-two-subnets-behind-nat-gateways/

2) Get two containers connected via this overlay network. The simplest place to
start would be to createa an interface alias for the openvpn tap device, give
the container the host networking namespace and then have it bind on that interface.

3) Write a thing that uses etcd to register machines preferred network ip for
every machine in the cluster to connect to. Machines in the network should
create a new openvpn connection for every registered machine and ensure it is
up.

4) Configure this whole thing using etcd and hold the network keys in etcd too.

5) Ship it!
