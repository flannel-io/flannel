# Static Configuration Guide

This guide will demonstrate how to run flannel with a static configuration and subnet lease.

Why would you want to do this?

Running both flannel and etcd from docker containers presents a bootstrapping problem if you want etcd to reside within the flannel overlay network.
Using a static configuration and subnet lease you can start flannel before etcd is up.

The high level steps required to make this work are as follows:

* select a subnet range and reserve 1 or more subnets for static assignment (hint: use the SubnetMin flannel configuration setting) 
* start flannel using the `-config` and `-subnet` flags
* start docker after flannel
* start etcd using docker

When starting flannel you will need to know the IP address of the docker container to use as the `etcd-endpoint`. Using something like DNS or a Kubernetes service IP can help here.

## Starting flannel with a static configuration and subnet lease

```
flanneld -config '{"Network": "10.0.0.0/8", "SubnetLen": 20, "SubnetMin": "10.10.0.4", "SubnetMax": "10.99.0.0"}' \
  -subnet="10.10.0.0-24" \
  -etcd-endpoints http://192.168.0.1:2379
```

Once flannel is up and running, flannel will attempt to register its lease with etcd every minute until it works. Expect to see the following log entries during this process:

```
E0225 06:45:14.164910 03447 subnet.go:474] Error renewing lease (trying again in 1 min): 501: All the given peers are not reachable (Tried to connect to each peer twice and failed) [0]
E0225 06:45:14.672713 03447 subnet.go:405] Watch of subnet leases failed: 501: All the given peers are not reachable (Tried to connect to each peer twice and failed) [0]
E0225 06:45:15.165571 03447 subnet.go:474] Error renewing lease (trying again in 1 min): 501: All the given peers are not reachable (Tried to connect to each peer twice and failed) [0]
```

Once etcd is online and flannel registers its static lease, you'll see the following message in your logs:

```
I0225 06:45:17.334540 03447 subnet.go:480] Lease renewed, new expiration: 2015-02-26 14:45:16.215118532 +0000 UTC
```
