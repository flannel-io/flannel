# Configuration

If the --kube-subnet-mgr argument is true, flannel reads its configuration from `/etc/kube-flannel/net-conf.json`.

If the --kube-subnet-mgr argument is false, flannel reads its configuration from etcd.
By default, it will read the configuration from `/coreos.com/network/config` (which can be overridden using `--etcd-prefix`).

Use the `etcdctl` utility to set values in etcd.

The value of the config is a JSON dictionary with the following keys:

* `Network` (string): IPv4 network in CIDR format to use for the entire flannel network. (This is the only mandatory key.)

* `IPv6Network` (string): IPv6 network in CIDR format to use for the entire flannel network. (Mandatory if EnableIPv6 is true)

* `EnableIPv6` (bool): Enables ipv6 support

* `SubnetLen` (integer): The size of the subnet allocated to each host.
   Defaults to 24 (i.e. /24) unless `Network` was configured to be smaller than a /22 in which case it is two less than the network.

* `SubnetMin` (string): The beginning of IP range which the subnet allocation should start with.
   Defaults to the second subnet of `Network`.

* `SubnetMax` (string): The end of the IP range at which the subnet allocation should end with.
   Defaults to the last subnet of `Network`.

* `IPv6SubnetLen` (integer): The size of the ipv6 subnet allocated to each host.
   Defaults to 64 (i.e. /64) unless `Ipv6Network` was configured to be smaller than a /62 in which case it is two less than the network.

* `IPv6SubnetMin` (string): The beginning of IPv6 range which the subnet allocation should start with.
   Defaults to the second subnet of `Ipv6Network`.

* `IPv6SubnetMax` (string): The end of the IPv6 range at which the subnet allocation should end with.
   Defaults to the last subnet of `Ipv6Network`.

* `Backend` (dictionary): Type of backend to use and specific configurations for that backend.
   The list of available backends and the keys that can be put into the this dictionary are listed in [Backends](backends.md).
   Defaults to `vxlan` backend.

Subnet leases have a duration of 24 hours. Leases are renewed within 1 hour of their expiration,
unless a different renewal margin is set with the ``--subnet-lease-renew-margin`` option.

## Example configuration JSON

The following configuration illustrates the use of most options with `udp` backend.

```json
{
	"Network": "10.0.0.0/8",
	"SubnetLen": 20,
	"SubnetMin": "10.10.0.0",
	"SubnetMax": "10.99.0.0",
	"Backend": {
		"Type": "udp",
		"Port": 7890
	}
}
```

## Key command line options

```bash
--public-ip="": IP accessible by other nodes for inter-host communication. Defaults to the IP of the interface being used for communication.
--etcd-endpoints=http://127.0.0.1:4001: a comma-delimited list of etcd endpoints.
--etcd-prefix=/coreos.com/network: etcd prefix.
--etcd-keyfile="": SSL key file used to secure etcd communication.
--etcd-certfile="": SSL certification file used to secure etcd communication.
--etcd-cafile="": SSL Certificate Authority file used to secure etcd communication.
--kube-subnet-mgr: Contact the Kubernetes API for subnet assignment instead of etcd.
--iface="": interface to use (IP or name) for inter-host communication. Defaults to the interface for the default route on the machine. This can be specified multiple times to check each option in order. Returns the first match found.
--iface-regex="": regex expression to match the first interface to use (IP or name) for inter-host communication. If unspecified, will default to the interface for the default route on the machine. This can be specified multiple times to check each regex in order. Returns the first match found. This option is superseded by the iface option and will only be used if nothing matches any option specified in the iface options.
--iface-can-reach="": detect interface to use (IP or name) for inter-host communication based on which will be used for provided IP. This is exactly the interface to use of command "ip route get <ip-address>" (example: --iface-can-reach=192.168.1.1 results the interface can be reached to 192.168.1.1 will be selected)
--iptables-resync=5: resync period for iptables rules, in seconds. Defaults to 5 seconds, if you see a large amount of contention for the iptables lock increasing this will probably help.
--subnet-file=/run/flannel/subnet.env: filename where env variables (subnet and MTU values) will be written to.
--net-config-path=/etc/kube-flannel/net-conf.json: path to the network configuration file to use
--subnet-lease-renew-margin=60: subnet lease renewal margin, in minutes.
--ip-masq=false: setup IP masquerade for traffic destined for outside the flannel network. Flannel assumes that the default policy is ACCEPT in the NAT POSTROUTING chain.
-v=0: log level for V logs. Set to 1 to see messages related to data path.
--healthz-ip="0.0.0.0": The IP address for healthz server to listen (default "0.0.0.0")
--healthz-port=0: The port for healthz server to listen(0 to disable)
--version: print version and exit
```

MTU is calculated and set automatically by flannel. It then reports that value in `subnet.env`. This value cannot be changed.

## Environment variables

The command line options outlined above can also be specified via environment variables.
For example `--etcd-endpoints=http://10.0.0.2:2379` is equivalent to `FLANNELD_ETCD_ENDPOINTS=http://10.0.0.2:2379` environment variable.
Any command line option can be turned into an environment variable by prefixing it with `FLANNELD_`, stripping leading dashes, converting to uppercase and replacing all other dashes to underscores.

`EVENT_QUEUE_DEPTH` is another environment variable to indicate the kubernetes scale. Set `EVENT_QUEUE_DEPTH` to adapter your cluster node numbers. If not set, default value is 5000. 

## Health Check

Flannel provides a health check http endpoint `healthz`. Currently this endpoint will blindly
return http status ok(i.e. 200) when flannel is running. This feature is by default disabled.
Set `healthz-port` to a non-zero value will enable a healthz server for flannel.

## Dual-stack

Flannel supports dual-stack mode. This means pods and services could use ipv4 and ipv6 at the same time. Currently, dual-stack is only supported for vxlan, wireguard or host-gw(linux) backends.

Requirements:
* v1.0.1 of flannel binary from [containernetworking/plugins](https://github.com/containernetworking/plugins)
* Nodes must have an ipv4 and ipv6 address in the main interface
* Nodes must have an ipv4 and ipv6 address default route
* vxlan support ipv6 tunnel require kernel version >= 3.12

Configuration:
* Set "EnableIPv6": true and the "IPv6Network", for example "IPv6Network": * "2001:cafe:42:0::/56" in the net-conf.json of the kube-flannel-cfg ConfigMap or in `/coreos.com/network/config` for etcd

If everything works as expected, flanneld should generate a `/run/flannel/subnet.env` file with IPV6 subnet and network. For example:

```bash
FLANNEL_NETWORK=10.42.0.0/16
FLANNEL_SUBNET=10.42.0.1/24
FLANNEL_IPV6_NETWORK=2001:cafe:42::/56
FLANNEL_IPV6_SUBNET=2001:cafe:42::1/64
FLANNEL_MTU=1450
FLANNEL_IPMASQ=true
```

## IPv6 only

To use an IPv6 only environment use the same configuration of the Dual-stack to enable IPv6 and add "EnableIPv4": false in the net-conf.json of the kube-flannel-cfg ConfigMap
