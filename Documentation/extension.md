The `extension` backend provides an easy way for prototyping new backend types for flannel.

It is _not_ recommended for production use, for example it doesn't have a built in retry mechanism.

This backend has the following configuration
* `Type` (string): `extension`
* `PreStartupCommand`  (string): Command to run before allocating a network to this host
    * The stdout of the process is captured and passed to the stdin of the SubnetAdd/Remove commands.
* `PostStartupCommand`  (string): Command to run after allocating a network to this host
    * The following environment variable is set
            * SUBNET - The subnet of the remote host that was added.
* `SubnetAddCommand`   (string): Command to run when a subnet is added
    * stdin - The output from `PreStartupCommand` is passed in.
    * The following environment variables are set
        * SUBNET - The subnet of the remote host that was added.
        * PUBLIC_IP - The public IP of the remote host.
* `SubnetRemoveCommand`(string): Command to run when a subnet is removed
    * stdin - The output from `PreStartupCommand` is passed in.
      * The following environment variables are set
          * SUBNET - The subnet of the remote host that was removed.
          * PUBLIC_IP - The public IP of the remote host.

All commands are run through the `sh` shell and are run with the same permissions as the flannel daemon.


## Simple example (host-gw)
To replicate the functionality of the host-gw plugin, there's no need for a startup command.

The backend just needs to manage the route to subnets when they are added or removed.

An example
```json
{
  "Network": "10.0.0.0/16",
  "Backend": {
    "Type": "extension",
    "SubnetAddCommand": "ip route add $SUBNET via $PUBLIC_IP",
    "SubnetRemoveCommand": "ip route del $SUBNET via $PUBLIC_IP"
  }
}
```


## Complex example (vxlan)
VXLAN is more complex. It needs to store the MAC address of the vxlan device when it's created and to make it available to the flannel daemon running on other hosts.
The address of the vxlan device also needs to be set _after_ the subnet has been allocated.

An example
```json
{
  "Network": "10.50.0.0/16",
  "Backend": {
    "Type": "extension",
    "PreStartupCommand": "export VNI=1; export IF_NAME=flannel-vxlan; ip link del $IF_NAME 2>/dev/null; ip link add $IF_NAME type vxlan id $VNI dstport 8472 && cat /sys/class/net/$IF_NAME/address",
    "PostStartupCommand": "export IF_NAME=flannel-vxlan; export SUBNET_IP=`echo $SUBNET | cut -d'/' -f 1`; ip addr add $SUBNET_IP/32 dev $IF_NAME && ip link set $IF_NAME up",
    "SubnetAddCommand": "export SUBNET_IP=`echo $SUBNET | cut -d'/' -f 1`; export IF_NAME=flannel-vxlan; read VTEP; ip route add $SUBNET nexthop via $SUBNET_IP dev $IF_NAME onlink && arp -s $SUBNET_IP $VTEP dev $IF_NAME && bridge fdb add $VTEP dev $IF_NAME self dst $PUBLIC_IP"
  }
}
```