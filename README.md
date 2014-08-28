# Rudder

Rudder is an overlay network that gives a subnet to each machine for use with
Kubernetes.

In Kubernetes every machine in the cluster is assigned a full subnet. The machine A
and B might have 10.0.1.0/24 and 10.0.2.0/24 respectively. The advantage of
this model is that it reduces the complexity of doing port mapping. The
disadvantage is that the only cloud provider that can do this is GCE.

## Theory of Operation

To emulate the Kubernetes model from GCE on other platforms we need to create
an overlay network on top of the network that we are given from cloud
providers. Rudder uses the Universal TUN/TAP device and creates an overlay network
using UDP to encapsulate IP packets. The subnet allocation is done with the help
of etcd which maintains the overlay to actual IP mappings.

## Building Rudder

* Step 1: Make sure you have Linux headers installed on your machine. On Ubuntu, run ```sudo apt-get install linux-libc-dev```. On Fedora/Redhat, run ```sudo yum install kernel-headers```.
* Step 2: Git clone the Rudder repo: ```git clone git@github.com:coreos/rudder.git```
* Step 3: Run the build script: ```cd rudder; ./build```

## Configuration

Rudder reads its configuration from etcd. By default, it will read the configuration
from ```/coreos.com/network/config``` (can be overridden via --etcd-prefix).
The value of the config should be a JSON dictionary with the following keys:

* ```Network``` (string): IPv4 network in CIDR format to use for the entire overlay network. This
is the only mandatory key.

* ```SubnetLen``` (number): The size of the subnet allocated to each host. Defaults to 24 (i.e. /24) unless
the Network was configured to be smaller than a /24 in which case it is one less than the network.

* ```SubnetMin``` (string): The beginning of IP range which the subnet allocation should start with. Defaults
to the first subnet of Network.

* ```SubnetMax``` (string): The end of the IP range at which the subnet allocation should end with. Defaults to
the last subnet of Network.

## Running

Once you have pushed configuration JSON to etcd, you can start Rudder. If you published your
config at the default location, you can start Rudder with no arguments. Rudder will acquire a
subnet lease, configure its routes based on other leases in the overlay network and start
routing packets. Additionally it will monitor etcd for new members of the network and adjust
its routing table accordingly.

After Rudder has acquired the subnet and configured the TUN device, it will write out an
environment variable file (```/run/rudder/subnet.env``` by default) with subnet address and
MTU that it supports.

## Key command line options

```
-etcd-endpoint="http://127.0.0.1:4001": etcd endpoint
-etcd-prefix="/coreos.com/network": etcd prefix
-iface="": interface to use (IP or name) for inter-host communication. Defaults to the interface for the default route on the machine.
-port=8285: UDP port to use for inter-node communications
-subnet-file="/run/rudder/subnet.env": filename where env variables (subnet and MTU values) will be written to
-v=0: log level for V logs. Set to 1 to see messages related to data path
```

## Docker integration

Docker daemon accepts ```--bip``` argument to configure the subnet of the docker0 bridge. It also accepts ```--mtu``` to set the MTU
for docker0 and veth devices that it will be creating. Since Rudder writes out the acquired subnet and MTU values into
a file, the script starting Docker daemon can source in the values and pass them to Docker daemon:

```bash
source /run/rudder/subnet.env
docker -d --bip=${RUDDER_SUBNET} --mtu=${RUDDER_MTU}
```

Systemd users can use ```EnvironmentFile``` directive in the .service file to pull in ```/run/rudder/subnet.env```

## CoreOS integration

On CoreOS it is useful to add Rudder configuration into .service file in the cloud-config as the following snippet demonstrates:

```
  - name: rudder.service
    command: start
    content: |
      [Unit]
      Requires=etcd.service
      After=etcd.service

      [Service]
      ExecStartPre=-/usr/bin/etcdctl mk /coreos.com/network/config '{"Network":"10.0.0.0/16"}'
      ExecStart=/opt/bin/rudder
```
