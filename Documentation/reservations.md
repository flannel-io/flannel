# Leases and Reservations

## Leases

When flannel starts up, it ensures that the host has a subnet lease. If there is
an existing lease then it's used, otherwise one is assigned.

Leases can be viewed by checking the contents of etcd. e.g.

```
$ etcdctl ls /coreos.com/network/subnets            
/coreos.com/network/subnets/10.5.34.0-24
$ etcdctl -o extended get /coreos.com/network/subnets/10.5.34.0-24
Key: /coreos.com/network/subnets/10.5.34.0-24
Created-Index: 5
Modified-Index: 5
TTL: 85925
Index: 6

{"PublicIP":"10.37.7.195","BackendType":"vxlan","BackendData":{"VtepMAC":"82:4b:b6:2f:54:4e"}}
```

This shows that there is a single lease (`10.5.34.0/24`) which will expire in 85925 seconds. flannel will attempt to renew the lease before it expires, but if flannel is not running for an extended period then the lease will be lost.

The `"PublicIP"` value is how flannel knows to reuse this lease when restarted. 
This means that if the public IP changes, then the flannel subnet will change too.

In case a host is unable to renew its lease before the lease expires (e.g. a host takes a long time to restart and the timing lines up with when the lease would normally be renewed), flannel will then attempt to renew the last lease that it has saved in its subnet config file (which, unless specified, is located at `/var/run/flannel/subnet.env`)
```bash
cat /var/run/flannel/subnet.env
FLANNEL_NETWORK=10.5.0.0/16
FLANNEL_SUBNET=10.5.34.1/24
FLANNEL_MTU=1450
FLANNEL_IPMASQ=false
```
In this case, if flannel fails to retrieve an existing lease from etcd, it will attempt to renew lease specified in `FLANNEL_SUBNET` (`10.5.34.1/24`).  It will only renew this lease if the subnet specified is valid for the current etcd network configuration otherwise it will allocate a new lease.

## Reservations

flannel also supports reservations for the subnet assigned to a host. Reservations
allow a fixed subnet to be used for a given host.

The only difference between a lease and reservation is the etcd TTL value. Simply 
removing the TTL from a lease will convert it to a reservation. e.g.

```
etcdctl set -ttl 0 /coreos.com/network/subnets/10.5.1.0-24 $(etcdctl get /coreos.com/network/subnets/10.5.1.0-24)
```
