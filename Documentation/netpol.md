# Network policy controller

From v0.25.5 it is possible to deploy Flannel with https://github.com/kubernetes-sigs/kube-network-policies controller to provide a network policy controller within the Flannel CNI.

When deployed with the Helm chart it is enough to enable the `netpol.enabled` value.
```bash
helm install flannel --set netpol.enabled=true --namespace kube-flannel flannel/flannel
```

Flannel pod should start with an additional container and it is possible to configure Network policies.
Use the kube-network-poilicies documentation to find additional info.
