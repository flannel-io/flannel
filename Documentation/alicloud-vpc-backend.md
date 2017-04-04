# AliCloud VPC Backend for Flannel

When running in an AliCloud VPC, we recommend using the ali-vpc backend to build your network. This prevent from package encapsulation compare to overlay network and achieve maximum performance with IP routes. Also, there is no need for an separate flannel interface to be created.
In order to run flannel on AliCloud we need first create an [AliCloud VPC Network](https://vpc.console.aliyun.com/#/vpc/cn-hangzhou/list)

### Create VPC network
Navigate to AliCloud VPC Network list page, then click [create vpc network] button.
![vpc](img/ali-create-vpc.png)

- Set vpc name with some meaningful string.
- Choose a subnet for the VPC. There are three subnet for you to select which is 192.168.0.0/16 172.16.0.0/12 10.0.0.0/8. Choose one according to your cluster size.
- Click create and wait for ready.

### Create switch
Click manager switch to navigate to switch list page, and create a switch.

- Set switch name to a meaningful string.
- Choose one AV Zone where you want to run your ECS
- Set up a subnet which should be contained in your VPC subnet. Here we set subnet as 192.168.0.0/16.
- Confirm Creating.

### Create instance
Create an instance whose network type is VPC and then add the instance to your previous VPC network. Note: The ECS you created must sit in the same AV zone with your previous created switch.
![create instance](img/ali-create-instance.png)

- Select the proper VPC network.

### Get your own ACCESS_KEY_ID and ACCESS_KEY_SECRET. 
Click [find key](https://ak-console.aliyun.com/#/accesskey)

![create key](img/ali-create-key.png)

- If you havent create any key yet, just click [create key secret] to create a new one.
- take a note of AccessKeyId and AccessKeySecret for further use.

### Go ahead and launch the instance! 

All that’s left now is to start etcd, publish the network configuration and run the flannel daemon. 
First, SSH into `instance-1`:

- Start etcd:

```
$ etcd2 -advertise-client-urls http://$INTERNAL_IP:2379 -listen-client-urls http://0.0.0.0:2379
```
- Publish configuration in etcd (ensure that the network range does not overlap with the one configured for the VPC)

```
$ etcdctl set /coreos.com/network/config '{"Network":"10.24.0.0/16", "Backend": {"Type": "ali-vpc"}}'
```
- Fetch the latest release using wget from https://github.com/coreos/flannel/ 
- make dist/flanneld
- Run flannel daemon:

```
sudo ./flanneld --etcd-endpoints=http://127.0.0.1:2379
```

Next, create and connect to a clone of `instance-1`.
Run flannel with the `--etcd-endpoints` flag set to the *internal* IP of the instance running etcd.

Confirm that the subnet route table has entries for the lease acquired by each of the subnets.

![router-confirm](img/ali-vpc-confirm.png)
### Limitations

Keep in mind that the AliCloud VPC [limits](https://vpc.console.aliyun.com/#/vpc/cn-hangzhou/detail/vpc-bp11xpfe5ev6wvhfb14b6/router) the number of entries per route table to 50. If you require more routes, request a quota increase or simply switch to the VXLAN backend.
