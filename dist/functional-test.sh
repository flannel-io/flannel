#!/bin/bash

CNI_NET="10.100.0.0/16"
CNI_GW="10.100.0.1"

ETCD_IMG="coreos.com/etcd:v3.0.3"
ETCD_ENDPT="http://$CNI_GW:2379"

FLANNEL_NET="10.10.0.0/16"



usage() {
	echo "$0 FLANNEL-ACI-FILE | FLANNEL-IMAGE-NAME"
	echo
	echo "Run end-to-end tests by bringing up two flannel instances"
	echo "and having them ping each other"
	echo
	echo "NOTE: this script depends on rkt, actool and jq"
	exit 1
}

# Generate a random alpha-numeric string of given length
rand_string() {
	len=$1
	cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w $1 | head -n 1
}

# Get the ACName of the flannel image by analyzing $1
get_flannel_img() {
	filename=$(basename "$1")
	extension="${filename##*.}"

	if [ "$extension" == "aci" ]; then
		if ! rkt --insecure-options=image fetch "$1" >/dev/null; then
			exit 1
		fi

		# parse out the ACI name out of its manifest
		aci_manifest=$(actool cat-manifest $1)
		aci_name=$(echo $aci_manifest | jq -r .name)
		aci_version=$(echo $aci_manifest | jq -r '.labels[] | select(.name=="version") | .value')
		echo "${aci_name}:${aci_version}"
	else
		echo $1
	fi
}

run_test() {
	backend=$1

	while true; do
		# etcd might take a bit to come up
		etcdctl=$(rkt --insecure-options=image prepare --stage1-name=coreos.com/rkt/stage1-fly:1.11.0 $ETCD_IMG \
			--volume data-dir,kind=host,source=/tmp \
			--exec /etcdctl -- \
			--endpoints=$ETCD_ENDPT set /coreos.com/network/config "{ \"Network\": \"$FLANNEL_NET\", \"Backend\": { \"Type\": \"${backend}\" } }")

		rkt run-prepared $etcdctl
		ok=$?

		rkt rm $etcdctl

		if [ $ok -eq 0 ]; then
			break
		fi

		sleep 5
	done

	echo flannel config written

	# rkt (via systemd-nspawn) mounts /proc/sys ro so this needs to be changed back to rw
	flannel_sh_exec_args="mount -o remount,rw /proc/sys && /opt/bin/flanneld --etcd-endpoints=$ETCD_ENDPT --iface=eth0" 
	flannel_vols="--volume etc-ssl-etcd,kind=host,source=/etc/ssl --volume dev-net,kind=host,source=/dev/net"
	# need CAP_SYS_ADMIN for mount
	img_opts="$flannel_vols --cap-retain CAP_SYS_ADMIN,CAP_NET_ADMIN --exec /bin/sh"

	flannel1=$(rkt --insecure-options=image prepare $flannel_img $img_opts -- -c "$flannel_sh_exec_args")
	if [ $? -ne 0 ]; then
		exit 1
	fi

	flannel2=$(rkt --insecure-options=image prepare $flannel_img $img_opts -- -c "$flannel_sh_exec_args")
	if [ $? -ne 0 ]; then
		exit 1
	fi

	echo flannel containers prepared

	rkt run-prepared --interactive --net=${cni_name},default $flannel1 &
	rkt run-prepared --interactive --net=${cni_name},default $flannel2 &

	echo flannels running
	sleep 5

	# add a dummy interface with $FLANNEL_SUBNET so we have a known working IP to ping
	rkt enter $flannel1 /bin/sh -c 'source /run/flannel/subnet.env; ip link add name dummy0 type dummy; ip addr add $FLANNEL_SUBNET dev dummy0; ip link set dummy0 up'
	ping_dest=$(rkt enter $flannel1 /bin/sh -c 'source /run/flannel/subnet.env; echo $FLANNEL_SUBNET | cut -f 1 -d "/" ')

	rkt enter $flannel2 /bin/ping -c 5 $ping_dest
	exit_code=$?

	# Uncomment to debug (you can nsenter)
	#if [ $exit_code -eq "1" ]; then
	#	sleep 10000
	#fi

	echo "Test for backend=$backend: exit=$exit_code"

	for fl in $flannel2 $flannel1; do
		rkt stop --force $fl
		#rkt rm $fl
	done

	return $exit_code
}

if [ $# -ne 1 ]; then
	usage
fi

cni_name="br$(rand_string 8)"
cni_conf=" 
{
	\"name\": \"$cni_name\",
	\"type\": \"bridge\",
	\"isGateway\": true,
	\"ipam\": {
		\"type\": \"host-local\",
		\"subnet\": \"$CNI_NET\"
	}
}
"

# Required by get_flannel_img to parse out the ACName
which jq >/dev/null || (echo "No jq utility found, please install" >/dev/stderr && exit 1)
which rkt >/dev/null || (echo "No rkt found, please install" >/dev/stderr && exit 1)
which actool >/dev/null || (echo "No actool found, please install" >/dev/stderr && exit 1)

flannel_img=$(get_flannel_img $1)

# Write out the CNI config
echo $cni_conf >/etc/rkt/net.d/${cni_name}.conf

# HACK: Bring up some container so the CNI network creates the bridge
rkt --insecure-options=image run --interactive --net=${cni_name} docker://busybox --exec "/bin/true"

etcd=$(rkt prepare $ETCD_IMG --port=client:2379 -- --listen-client-urls http://0.0.0.0:2379 --advertise-client-urls $ETCD_ENDPT)
if [ $? -ne 0 ]; then
	exit 1
fi

rkt run-prepared --interactive $etcd &

echo etcd launched

global_exit_code=0

backends="udp vxlan host-gw"

for backend in $backends; do
	echo "Running test for backend=$backend"

	run_test $backend
	if [ $? -ne "0" ]; then
		global_exit_code=1
	fi
done

rkt stop --force $etcd
#rkt rm $etcd

rm /etc/rkt/net.d/${cni_name}.conf

if [ $global_exit_code == 0 ]; then
	echo "ALL TESTS PASSED"
else
	echo "TEST(S) FAILED"
fi

exit $global_exit_code
