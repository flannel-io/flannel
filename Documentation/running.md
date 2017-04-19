## Running

Once you have pushed configuration JSON to etcd, you can start flanneld.
If you published your config at the default location, you can start flanneld with no arguments.
flannel will acquire a subnet lease, configure its routes based on other leases in the overlay network and start routing packets.
Additionally it will monitor etcd for new members of the network and adjust the routes accordingly.

After flannel has acquired the subnet and configured backend, it will write out an environment variable file (`/run/flannel/subnet.env` by default) with subnet address and MTU that it supports.

## Multiple networks

Flanneld does not support multiple from a single daemon (it did previously as an experimental feature).
However, it does support running multiple daemons on the same host with different configuration. The `-subnet-file` and `-etcd-prefix` options should be used to "namespace" the different daemons.
For example
```
flanneld -subnet-file /vxlan.env -etcd-prefix=/vxlan/network
```

## Zero-downtime restarts

When running with a backend other than `udp`, the kernel is providing the data path with flanneld acting as the control plane.
As such, flanneld can be restarted (even to do an upgrade) without disturbing existing flows.
However in the case of `vxlan` backend, this needs to be done within a few seconds as ARP entries can start to timeout requiring the flannel daemon to refresh them.
Also, to avoid interruptions during restart, the configuration must not be changed (e.g. VNI, --iface values).

## Docker integration

Docker daemon accepts `--bip` argument to configure the subnet of the docker0 bridge.
It also accepts `--mtu` to set the MTU for docker0 and veth devices that it will be creating.
Since flannel writes out the acquired subnet and MTU values into a file, the script starting Docker can source in the values and pass them to Docker daemon:

```bash
source /run/flannel/subnet.env
docker daemon --bip=${FLANNEL_SUBNET} --mtu=${FLANNEL_MTU} &
```

Systemd users can use `EnvironmentFile` directive in the .service file to pull in `/run/flannel/subnet.env`

## CoreOS integration

CoreOS ships with flannel integrated into the distribution.
See https://coreos.com/docs/cluster-management/setup/flannel-config/ for more information.

## Running on Vagrant

Vagrant has a tendency to give the default interface (one with the default route) a non-unique IP (often 10.0.2.15).
This causes flannel to register multiple nodes with the same IP.
To work around this issue, use `--iface` option to specify the interface that has a unique IP.
If you're running on CoreOS, use cloud-config to set `coreos.flannel.interface` to `$public_ipv4`.
