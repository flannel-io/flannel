# Add nftables implementation to flannel

Date: 2024-02-01

## Status

Writing

## Context
At the moment, flannel uses iptables to mask and route packets.
Our implementation is  based on the library from coreos (https://github.com/coreos/go-iptables).

There are several issues with using iptables in flannel:
* performance: packets are matched using a list so performance is O(n).
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

## nftables library
* call the `nft` executable directly
* https://github.com/kubernetes-sigs/knftables

## Implementation steps
### alpha
* refactor current iptables code to better encapsulate iptables calls in the dedicated package
* remove code for managing multiple networks that was added for the deprecated MultiClusterCIDR feature?
** => this is optional but would help to simplify the code
* implement nftables mode that is the exact equivalent of the current iptables code
* add similar unit tests and e2e test coverage
### beta
* try to optimize the code using nftables-specific feature
* integrate the new flag in k3s


## Decision
