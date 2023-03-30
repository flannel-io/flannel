# Upgrade

Flannel upgrade/downgrade procedure
 
There are different ways of changing flannel version in the running cluster:
 
## Remove old resources definitions and install a new one.
* Pros: Cleanest way of managing resources of the flannel deployment and no manual validation required as long as no additional resources was created by administrators/operators
* Cons: Massive networking outage within a cluster during the version change

*1. Delete all the flannel resources using kubectl*
```bash
kubectl -n kube-flannel delete daemonset kube-flannel-ds
kubectl -n kube-flannel delete configmap kube-flannel-cfg
kubectl -n kube-flannel delete serviceaccount flannel
kubectl delete clusterrolebinding.rbac.authorization.k8s.io flannel
kubectl delete clusterrole.rbac.authorization.k8s.io flannel
kubectl delete namespace kube-flannel
```

*2. Install the newer version of flannel and reboot the nodes*

## On the fly version
* Pros: Less disruptive way of changing flannel version, easier to do
* Cons: Some version may have changes which can't be just replaced and may need resources cleanup and/or rename, manual resources comparison required

If the update is done from newer version as 0.20.2 it can be done using kubectl
```bash
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
```
In case of error on the labeling follow the previous way.

## Using the helm repository

From version 0.21.4 flannel is deployed on an helm repository at `https://flannel-io.github.io/flannel/` it will be possible to manage the update directly with helm.
```bash
helm upgrade flannel --set podCidr="10.244.0.0/16" --namespace kube-flannel flannel/flannel
```
