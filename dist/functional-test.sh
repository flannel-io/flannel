#!/bin/bash
# Uncomment to see what commands are being executed
#set -x

ETCD_IMG="quay.io/coreos/etcd:v3.0.3"
FLANNEL_NET="10.10.0.0/16"

usage() {
	echo "$0 FLANNEL-DOCKER-IMAGE"
	echo
	echo "Run end-to-end tests by bringing up two flannel instances"
	echo "and having them ping each other"
	echo
	echo "NOTE: this script depends on Docker 1.9.0 or higher"
	exit 1
}

version_check() {
	required=$1
	actual=$2
	err_prefix=$3

	req_maj=$(echo $required | cut -d . -f 1)
	req_min=$(echo $required | cut -d . -f 2)
	act_maj=$(echo $actual | cut -d . -f 1)
	act_min=$(echo $actual | cut -d . -f 2)

	if [ $act_maj -lt $req_maj ] || ( [ $act_maj -eq $req_maj ] && [ $act_min -lt $req_min ] ); then
		echo "$err_prefix: required=$required, found=$actual"
		exit 1
	fi
}

docker_version_check() {
	ver=$(docker version -f '{{.Server.Version}}')
	version_check "1.9" $ver
}

run_test() {
	backend=$1

	flannel_conf="{ \"Network\": \"$FLANNEL_NET\", \"Backend\": { \"Type\": \"${backend}\" } }"

	# etcd might take a bit to come up
	while ! docker run --rm -it $ETCD_IMG etcdctl --endpoints=$etcd_endpt set /coreos.com/network/config "$flannel_conf"
	do
		sleep 1
	done

	echo flannel config written

	# rm any old flannel container that maybe running, ignore error as it might not exist
	docker rm -f flannel-e2e-test-flannel1 2>/dev/null
	docker run --name=flannel-e2e-test-flannel1 -d --privileged $flannel_img --etcd-endpoints=$etcd_endpt
	if [ $? -ne 0 ]; then
		exit 1
	fi

	# rm any old flannel container that maybe running, ignore error as it might not exist
	docker rm -f flannel-e2e-test-flannel2 2>/dev/null
	docker run --name=flannel-e2e-test-flannel2 -d --privileged $flannel_img --etcd-endpoints=$etcd_endpt
	if [ $? -ne 0 ]; then
		exit 1
	fi

	echo flannels running

	# wait an arbitrary amount to have flannels come up
	sleep 5

	# add a dummy interface with $FLANNEL_SUBNET so we have a known working IP to ping
	ping_dest=$(docker "exec" --privileged flannel-e2e-test-flannel1 /bin/sh -c '\
		source /run/flannel/subnet.env && 
		ip link add name dummy0 type dummy && \
		ip addr add $FLANNEL_SUBNET dev dummy0 && \
	       	ip link set dummy0 up && \
		echo $FLANNEL_SUBNET | cut -f 1 -d "/" ')

	docker exec -it --privileged flannel-e2e-test-flannel2 /bin/ping -c 5 $ping_dest
	exit_code=$?

	# Uncomment to debug (you can nsenter)
	#if [ $exit_code -eq "1" ]; then
	#	sleep 10000
	#fi

	echo "Test for backend=$backend: exit=$exit_code"

	# Perf test - run iperf server on flannel1 and client on flannel2
	if [ $exit_code -eq 0 ]; then
        docker rm -f flannel-e2e-test-flannel1-iperf 2>/dev/null
        docker run -d --name flannel-e2e-test-flannel1-iperf --net=container:flannel-e2e-test-flannel1 mlabbe/iperf3
        docker run --rm --net=container:flannel-e2e-test-flannel2 mlabbe/iperf3 -c $ping_dest
 fi

	docker stop flannel-e2e-test-flannel1 flannel-e2e-test-flannel2 >/dev/null

	if [ $exit_code -ne 0 ]; then
		# Print flannel logs to help debug
		echo "------ flannel server (one being pinged) log -------"
		docker logs flannel-e2e-test-flannel1
		echo

		echo "------ flannel client (one doing the ping) log -------"
		docker logs flannel-e2e-test-flannel2
		echo
	fi

	docker rm flannel-e2e-test-flannel1 flannel-e2e-test-flannel2 >/dev/null

	return $exit_code
}

multi_test() {
	flannel_conf_vxlan='{"Network": "10.11.0.0/16", "Backend": {"Type": "vxlan"}}'
	flannel_conf_host_gw='{"Network": "10.12.0.0/16", "Backend": {"Type": "host-gw"}}'

	# etcd might take a bit to come up
	while ! docker run --rm -it $ETCD_IMG etcdctl --endpoints=$etcd_endpt set /vxlan/network/config "$flannel_conf_vxlan"
	do
		sleep 1
	done

	while ! docker run --rm -it $ETCD_IMG etcdctl --endpoints=$etcd_endpt set /hostgw/network/config "$flannel_conf_host_gw"
	do
		sleep 1
	done

	echo flannel config written

	for host in 1 2; do
    	echo "=== Creating Host: $host ==============================================="

        # rm any old flannel container that maybe running, ignore error as it might not exist
        docker rm -f flannel-host$host 2>/dev/null

        # Start the hosts
        docker run --name=flannel-host$host -d -it --privileged --entrypoint /bin/sh $flannel_img

        # Start two flanneld instances
        docker exec -d flannel-host$host sh -c "/opt/bin/flanneld -subnet-file /vxlan.env -etcd-prefix=/vxlan/network --etcd-endpoints=$etcd_endpt 2>vxlan.log"
        docker exec -d flannel-host$host sh -c "/opt/bin/flanneld -subnet-file /hostgw.env -etcd-prefix=/hostgw/network --etcd-endpoints=$etcd_endpt 2>hostgw.log"
    done

	echo flannels running

	# wait an arbitrary amount to have flannels come up
	sleep 1

	# add dummy interface on host1 only so we have a known working IP to ping then ping it from host2
	vxlan_ping_dest=$(docker exec flannel-host1 /bin/sh -c '\
		source /vxlan.env &&
		ip link add name dummy_vxlan type dummy && \
		ip addr add $FLANNEL_SUBNET dev dummy_vxlan && \
	       	ip link set dummy_vxlan up && \
		echo $FLANNEL_SUBNET | cut -f 1 -d "/" ')

    hostgw_ping_dest=$(docker exec flannel-host1 /bin/sh -c '\
		source /hostgw.env &&
		ip link add name dummy_hostgw type dummy && \
		ip addr add $FLANNEL_SUBNET dev dummy_hostgw && \
	       	ip link set dummy_hostgw up && \
		echo $FLANNEL_SUBNET | cut -f 1 -d "/" ')

    # Send some pings from host2. Make sure we can send traffic over vxlan or directly.
    # If a particular (wrong) interface is forced then pings should fail
	docker exec -it flannel-host2 ping -c 3 $hostgw_ping_dest && \
	docker exec -it flannel-host2 ping -c 3 $vxlan_ping_dest && \
	! docker exec -it flannel-host2 ping -W 1 -c 1 -I flannel.1 $hostgw_ping_dest && \
	! docker exec -it flannel-host2 ping -W 1 -c 1 -I eth0 $vxlan_ping_dest
	exit_code=$?

	# Uncomment to debug (you can nsenter)
	#if [ $exit_code -eq "1" ]; then
	#	sleep 10000
	#fi

	echo "Test for multi-backend: exit=$exit_code"


	if [ $exit_code -ne 0 ]; then
		# Print flannel logs to help debug
		echo "------ flannel server (one being pinged) log -------"
		docker exec flannel-host1 sh -c 'cat *.log'
		echo

		echo "------ flannel client (one doing the ping) log -------"
		docker exec flannel-host2 sh -c 'cat *.log'
		echo
	fi

	docker rm -f flannel-host1 flannel-host2 >/dev/null

	return $exit_code
}

if [ $# -ne 1 ]; then
	usage
fi

flannel_img=$1

# Check that docker is new enough
docker_version_check

docker0=$(ip -o -f inet addr show docker0 | grep -Po 'inet \K[\d.]+')
etcd_endpt="http://$docker0:2379"

docker rm -f flannel-e2e-test-etcd 2>/dev/null
docker run --name=flannel-e2e-test-etcd -d -p 2379:2379 $ETCD_IMG etcd --listen-client-urls http://0.0.0.0:2379 --advertise-client-urls $etcd_endpt
if [ $? -ne 0 ]; then
	exit 1
fi

echo etcd launched

global_exit_code=0

backends=${BACKEND:-"udp vxlan host-gw"}
for backend in $backends; do
	echo
	echo "=== BACKEND: $backend ==============================================="

	if ! run_test $backend; then
		global_exit_code=1
	fi
done

echo "=== MULTI BACKEND ==============================================="
if ! multi_test; then
    global_exit_code=1
fi

docker stop flannel-e2e-test-etcd >/dev/null

if [ $global_exit_code -eq 0 ]; then
	echo
	echo "ALL TESTS PASSED"
else
	# Print etcd logs to help debug
	echo "------ etcd log -------"
	docker logs $etcd
	echo
	echo "TEST(S) FAILED"
fi

docker rm flannel-e2e-test-etcd 2>/dev/null

exit $global_exit_code
