### Backends
* udp: use UDP to encapsulate the packets.
  * `Type` (string): `udp`
  * `Port` (number): UDP port to use for sending encapsulated packets. Defaults to 8285.

* vxlan: use in-kernel VXLAN to encapsulate the packets.
  * `Type` (string): `vxlan`
  * `VNI`  (number): VXLAN Identifier (VNI) to be used. Defaults to 1.
  * `Port` (number): UDP port to use for sending encapsulated packets. Defaults to kernel default, currently 8472.
  * `GBP` (boolean): Enable [VXLAN Group Based Policy](https://github.com/torvalds/linux/commit/3511494ce2f3d3b77544c79b87511a4ddb61dc89).  Defaults to false.

* host-gw: create IP routes to subnets via remote machine IPs.
  Note that this requires direct layer2 connectivity between hosts running flannel.
  * `Type` (string): `host-gw`

* aws-vpc: create IP routes in an [Amazon VPC route table](http://docs.aws.amazon.com/AmazonVPC/latest/UserGuide/VPC_Route_Tables.html).
  * Requirements:
	* Running on an EC2 instance that is in an Amazon VPC.
	* Permissions required: `CreateRoute`, `DeleteRoute`,`DescribeRouteTables`, `ModifyInstanceAttribute`, `DescribeInstances [optional]`
  * `Type` (string): `aws-vpc`
  * `RouteTableID` (string): [optional] The ID of the VPC route table to add routes to.
     The route table must be in the same region as the EC2 instance that flannel is running on.
     flannel can automatically detect the id of the route table if the optional `DescribeInstances` is granted to the EC2 instance.

  Authentication is handled via either environment variables or the node's IAM role.
  If the node has insufficient privileges to modify the VPC routing table specified, ensure that appropriate `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and optionally `AWS_SECURITY_TOKEN` environment variables are set when running the flanneld process.

  Note: Currently, AWS [limits](http://docs.aws.amazon.com/AmazonVPC/latest/UserGuide/VPC_Appendix_Limits.html) the number of entries per route table to 50.

* gce: create IP routes in a [Google Compute Engine Network](https://cloud.google.com/compute/docs/networking#networks)
  * Requirements:
    * [Enable IP forwarding for the instances](https://cloud.google.com/compute/docs/networking#canipforward).
    * [Instance service account](https://cloud.google.com/compute/docs/authentication#using) with read-write compute permissions.
  * `Type` (string): `gce`

  Command to create a compute instance with the correct permissions and IP forwarding enabled:
  `$ gcloud compute instances create INSTANCE --can-ip-forward --scopes compute-rw`

  Note: Currently, GCE [limits](https://cloud.google.com/compute/docs/resource-quotas) the number of routes for every *project* to 100.

* alloc: only perform subnet allocation (no forwarding of data packets).
  * `Type` (string): `alloc`

* ali-vpc: create IP routes in a [alicloud VPC route table](https://vpc.console.aliyun.com)
  * Requirements:
    * Running on an ECS instance that is in an Alicloud VPC.
    * Permission require accessid and keysecret
  * `Type` (string): `ali-vpc`
  * `AccessKeyID` (string): api access key id. can also be configure with environment ACCESS_KEY_ID
  * `AccessKeySecret` (string): api access key secret.can also be configure with environment ACCESS_KEY_SECRET
  Note: Currently, AliVPC limit the number of entries per route table to 50.
