# Add nftables implementation to flannel

Date: 2024-02-01

## Status

Writing

## Context
At the moment, flannel uses iptables to mask and route packets.
Our implementation is  based on the library from coreos (https://github.com/coreos/go-iptables).

There are several issues with using iptables in flannel:
* performance: packets are matched using a list so performance is O(n). This isn't very important for flannel because use few iptables rules anyway.
* stability: 
** rules must be purged then updated every time flannel needs to change a rule to keep the correct order
** there can be interferences with other k8s components using iptables as well (kube-proxy, kube-router...)
* deprecation: nftables is pushed as a replacement for iptables in the kernel and in future distros including the future RHEL.

References:
- https://github.com/kubernetes/enhancements/blob/master/keps/sig-network/3866-nftables-proxy/README.md#motivation

## Current state
In flannel code, all references to iptables are wrapped in the `iptables` package.

The package provides the type `IPTableRule` to represent an individual rule. This type is almost entirely internal to the package so it would be easy to refactor the code to hide in favor of a more abstract type that would work for both iptables and nftables rules.

Unfortunately the package doesn't provide an interface so in order to provide both an iptables-based and an nftables-based implementation this needs to be refactored.

This package includes several Go interfaces (`IPTables`, `IPTablesError`) that are used for testing.

## Requirements
Ideally, flannel will include both iptables and nftables implementation. These need to coexist in the code but will be mutually exclusive at runtime.

The choice of which implementation to use will be triggered by an optional CLI flag.
iptables will remain the default for the time being.

Using nftables is an opportunity for optimising the rules deployed by flannel but we need to be careful about retro-compatibility with the current backend.

Starting flannel in either mode should reset the other mode as best as possible to ensure that users don't need to reboot if they need to change mode.

## Architecture
Currently, flannel uses two dedicated tables for its own rules: `FLANNEL-POSTRTG` and `FLANNEL-FWD`.
* flannel adds rules to the `FORWARD` and `POSTROUTING` tables to direct traffic to its own tables.
* rules in `FLANNEL-POSTRTG` are used to manage masquerading of the traffic to/from the pods
* rules in `FLANNEL-FWD` are used to ensure that traffic to and from the flannel network can be forwarded

With nftables, flannel would have its own dedicated table (`flannel`) with arbitrary chains and rules as needed.

see https://wiki.nftables.org/wiki-nftables/index.php/Performing_Network_Address_Translation_(NAT)
```
# !! untested example
table flannel {
    chain flannel-postrtg {
        type nat hook postrouting priority 0; 
        # kube-proxy
        meta mark 0x4000/0x4000 return
        # don't NAT traffic within overlay network 
        ip saddr $pod_cidr ip daddr $cluster_cidr return 
        ip saddr $cluster_cidr ip daddr $pod_cidr return 
        # Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
        ip saddr != $pod_cidr ip daddr $cluster_cidr return 
        # NAT if it's not multicast traffic
        ip saddr $cluster_cidr ip daddr != 224.0.0.0/4 nat 
        # Masquerade anything headed towards flannel from the host
        ip saddr != $cluster_cidr ip daddr $cluster_cidr nat 
    }

    chain flannel-fwd {
        type filter hook input priority 0; policy drop;
        # allow traffic to be forwarded if it is to or from the flannel network range
        ip saddr flannelNetwork accept
        ip daddr flannelNetwork accept
    }
}
```

## nftables library
We can either:
* call the `nft` executable directly
* use https://github.com/kubernetes-sigs/knftables which is developed for kube-proxy and should cover our use case

## Implementation steps
* refactor current iptables code to better encapsulate iptables calls in the dedicated package
* implement nftables mode that is the exact equivalent of the current iptables code
* add similar unit tests and e2e test coverage
* try to optimize the code using nftables-specific feature
* integrate the new flag in k3s


## Decision
