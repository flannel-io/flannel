## Configuration

flannel reads its configuration from etcd.
By default, it will read the configuration from `/coreos.com/network/config` (can be overridden via `--etcd-prefix`).
You can use `etcdctl` utility to set values in etcd.
The value of the config is a JSON dictionary with the following keys:

* `Network` (string): IPv4 network in CIDR format to use for the entire flannel network.
This is the only mandatory key.

* `SubnetLen` (integer): The size of the subnet allocated to each host.
   Defaults to 24 (i.e. /24) unless the Network was configured to be smaller than a /24 in which case it is one less than the network.

* `SubnetMin` (string): The beginning of IP range which the subnet allocation should start with.
   Defaults to the first subnet of Network.

* `SubnetMax` (string): The end of the IP range at which the subnet allocation should end with.
   Defaults to the last subnet of Network.

* `Backend` (dictionary): Type of backend to use and specific configurations for that backend.
   The list of available backends and the keys that can be put into the this dictionary are listed below.
   Defaults to "udp" backend.

The lease on a subnet is hard-coded to 24h (see [`subnetTTL`](subnet/local_manager.go#L31)).
Subnet lease are renewed within 1h of their expiration (can be overridden via `--subnet-lease-renew-margin`).

### Example configuration JSON

The following configuration illustrates the use of most options with `udp` backend.

```
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

```
--public-ip="": IP accessible by other nodes for inter-host communication. Defaults to the IP of the interface being used for communication.
--etcd-endpoints=http://127.0.0.1:4001: a comma-delimited list of etcd endpoints.
--etcd-prefix=/coreos.com/network: etcd prefix.
--etcd-keyfile="": SSL key file used to secure etcd communication.
--etcd-certfile="": SSL certification file used to secure etcd communication.
--etcd-cafile="": SSL Certificate Authority file used to secure etcd communication.
--kube-subnet-mgr: Contact the Kubernetes API for subnet assignement instead of etcd or flannel-server.
--iface="": interface to use (IP or name) for inter-host communication. Defaults to the interface for the default route on the machine.
--subnet-file=/run/flannel/subnet.env: filename where env variables (subnet and MTU values) will be written to.
--subnet-lease-renew-margin=60: subnet lease renewal margin, in minutes.
--ip-masq=false: setup IP masquerade for traffic destined for outside the flannel network. Flannel assumes that the default policy is ACCEPT in the NAT POSTROUTING chain.
--networks="": if specified, will run in multi-network mode. Value is comma separate list of networks to join.
-v=0: log level for V logs. Set to 1 to see messages related to data path.
--version: print version and exit
```

## Environment variables
The command line options outlined above can also be specified via environment variables.
For example `--etcd-endpoints=http://10.0.0.2:2379` is equivalent to `FLANNELD_ETCD_ENDPOINTS=http://10.0.0.2:2379` environment variable.
Any command line option can be turned into an environment variable by prefixing it with `FLANNELD_`, stripping leading dashes, converting to uppercase and replacing all other dashes to underscores.