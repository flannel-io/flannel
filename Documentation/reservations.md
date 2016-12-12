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

## Reservations

flannel also supports reservations for the subnet assigned to a host. Reservations
allow a fixed subnet to be used for a given host.

The only difference between a lease and reservation is the etcd TTL value. Simply 
removing the TTL from a lease will convert it to a reservation. e.g.

```
etcdctl set -ttl 0 /coreos.com/network/subnets/10.5.1.0-24 $(etcdctl get /coreos.com/network/subnets/10.5.1.0-24)
```
