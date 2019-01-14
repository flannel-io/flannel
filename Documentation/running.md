# Running flannel

Once you have pushed configuration JSON to `etcd`, you can start `flanneld`. If you published your config at the default location, you can start `flanneld` with no arguments.

Flannel will acquire a subnet lease, configure its routes based on other leases in the overlay network and start routing packets.

It will also monitor `etcd` for new members of the network and adjust the routes accordingly.

After flannel has acquired the subnet and configured backend, it will write out an environment variable file (`/run/flannel/subnet.env` by default) with subnet address and MTU that it supports.

For more information on checking the IP range for a specific host, see [Leases and Reservations][leases].

## Multiple networks

Flanneld does not support running multiple networks from a single daemon (it did previously as an experimental feature).
However, it does support running multiple daemons on the same host with different configurations. The `-subnet-file` and `-etcd-prefix` options should be used to "namespace" the different daemons.
For example
```
flanneld -subnet-file /vxlan.env -etcd-prefix=/vxlan/network
```

## Running manually

1. Download a `flannel` binary.
```bash
wget https://github.com/coreos/flannel/releases/download/v0.10.0/flanneld-amd64 && chmod +x flanneld-amd64
```
2. Run the binary.
```bash
sudo ./flanneld-amd64 # it will hang waiting to talk to etcd
```
3. Run `etcd`. Follow the instructions on the [CoreOS etcd page][coreos-etcd], or, if you have docker just do
```bash
docker run --rm --net=host quay.io/coreos/etcd
```
4. Observe that `flannel` can now talk to `etcd`, but can't find any config. So write some config. Either get `etcdctl` from the [CoreOS etcd page][coreos-etcd], or use `docker` again.
```bash
docker run --rm --net=host quay.io/coreos/etcd etcdctl set /coreos.com/network/config '{ "Network": "10.5.0.0/16", "Backend": {"Type": "vxlan"}}'
```
Now `flannel` is running, it has created a VXLAN tunnel device on the host and written a subnet config file

```bash
cat /run/flannel/subnet.env
FLANNEL_NETWORK=10.5.0.0/16
FLANNEL_SUBNET=10.5.72.1/24
FLANNEL_MTU=1450
FLANNEL_IPMASQ=false
```
Each time flannel is restarted, it will attempt to access the `FLANNEL_SUBNET` value written in this subnet config file. This prevents each host from needing to update its network information in case a host is unable to renew its lease before it expires (e.g. a host was restarting during the time flannel would normally renew its lease).

The `FLANNEL_SUBNET` value is also only used if it is valid for the etcd network config. For instance, a `FLANNEL_SUBNET` value of `10.5.72.1/24` will not be used if the etcd network value is set to `10.6.0.0/16` since it is not within that network range.

Subnet config value is `10.5.72.1/24`
```bash
cat /run/flannel/subnet.env
FLANNEL_NETWORK=10.5.0.0/16
FLANNEL_SUBNET=10.5.72.1/24
FLANNEL_MTU=1450
FLANNEL_IPMASQ=false
```
etcd network value is `10.6.0.0/16`. Since `10.5.72.1/24` is outside of this network, a new lease will be allocated.
```bash
etcdctl get /coreos.com/network/config
{ "Network": "10.6.0.0/16", "Backend": {"Type": "vxlan"}}
```

## Interface selection

Flannel uses the interface selected to register itself in the datastore.

The important options are:
* `-iface string`: Interface to use (IP or name) for inter-host communication.
* `-public-ip string`: IP accessible by other nodes for inter-host communication.

The combination of the defaults, the autodetection and these two flags ultimately result in the following being determined:
* An interface (used for MTU detection and selecting the VTEP MAC in VXLAN).
* An IP address for that interface.
* A public IP that can be used for reaching this node. In `host-gw` it should match the interface address.

## Making changes at runtime

Please be aware of the following flannel runtime limitations.
* The datastore type cannot be changed.
* The backend type cannot be changed. (It can be changed if you stop all workloads and restart all flannel daemons.)
* You can change the subnetlen/subnetmin/subnetmax with a daemon restart. (Subnets can be changed with caution. If pods are already using IP addresses outside the new range they will stop working.)
* The clusterwide network range cannot be changed (without downtime).

## Docker integration

Docker daemon accepts `--bip` argument to configure the subnet of the docker0 bridge.
It also accepts `--mtu` to set the MTU for docker0 and veth devices that it will be creating.

Because flannel writes out the acquired subnet and MTU values into a file, the script starting Docker can source in the values and pass them to Docker daemon:
```bash
source /run/flannel/subnet.env
docker daemon --bip=${FLANNEL_SUBNET} --mtu=${FLANNEL_MTU} &
```

Systemd users can use `EnvironmentFile` directive in the `.service` file to pull in `/run/flannel/subnet.env`

## CoreOS integration

CoreOS ships with flannel integrated into the distribution.
See [Configuring flannel for container networking][configuring-flannel] for more information.

## Running on Vagrant

Vagrant has a tendency to give the default interface (one with the default route) a non-unique IP (often 10.0.2.15).

This causes flannel to register multiple nodes with the same IP.

To work around this issue, use `--iface` option to specify the interface that has a unique IP.

If you're running on CoreOS, use `cloud-config` to set `coreos.flannel.interface` to `$public_ipv4`.

## Zero-downtime restarts

When running with a backend other than `udp`, the kernel is providing the data path with `flanneld` acting as the control plane.

As such, `flanneld` can be restarted (even to do an upgrade) without disturbing existing flows.

However in the case of `vxlan` backend, this needs to be done within a few seconds as ARP entries can start to timeout requiring the flannel daemon to refresh them.

Also, to avoid interruptions during restart, the configuration must not be changed (e.g. VNI, --iface values).


[coreos-etcd]: https://github.com/coreos/etcd/blob/master/Documentation/dev-guide/local_cluster.md
[configuring-flannel]: https://coreos.com/docs/cluster-management/setup/flannel-config/
[leases]: reservations.md
