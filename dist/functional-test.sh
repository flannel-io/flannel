#!/bin/bash

#Add "set -xe" to get more information where the unit test fail
set -xe

ARCH="${ARCH:-amd64}"
ETCD_IMG="${ETCD_IMG:-quay.io/coreos/etcd:v3.5.1}"
# etcd might take a bit to come up - use a known etcd version so we know we have etcdctl available
ETCDCTL_IMG="quay.io/coreos/etcd:v3.5.1"
ETCD_LOCATION="${ETCD_LOCATION:-etcd}"
FLANNEL_NET="${FLANNEL_NET:-10.10.0.0/16}"
TAG=`git describe --tags --dirty --always`
FLANNEL_DOCKER_IMAGE="${FLANNEL_DOCKER_IMAGE:-quay.io/coreos/flannel:$TAG}"

# Set the proper imagename according to architecture
if [[ ${ARCH} == "ppc64le" ]]; then
    ETCD_IMG+="-ppc64le"
    ETCDCTL_IMG+="-ppc64le"
elif [[ ${ARCH} == "arm64" ]]; then
    ETCD_IMG+="-arm64"
    ETCDCTL_IMG+="-arm64"
fi

setup_suite() {
    # Run etcd, killing any existing one that was running
    docker_ip=$(ip -o -f inet addr show docker0 | grep -Po 'inet \K[\d.]+')
    etcd_endpt="http://$docker_ip:2379"

    # Start etcd
    docker rm -f flannel-e2e-test-etcd >/dev/null 2>/dev/null
    docker run --name=flannel-e2e-test-etcd -d --dns 8.8.8.8 -v "${PWD}/test:/certs" \
    -e ETCD_UNSUPPORTED_ARCH=${ARCH} -p 2379:2379 $ETCD_IMG $ETCD_LOCATION \
    --listen-client-urls http://0.0.0.0:2379 \
    --cert-file=/certs/server.pem \
    --key-file=/certs/server-key.pem \
    --client-cert-auth \
    --trusted-ca-file=/certs/ca.pem \
    --advertise-client-urls $etcd_endpt >/dev/null
}

teardown_suite() {
    # Teardown the etcd server
    docker rm -f flannel-e2e-test-etcd >/dev/null
}

setup() {
    # rm any old flannel container that maybe running, ignore error as it might not exist
    docker rm -f flannel-e2e-test-flannel1 >/dev/null 2>/dev/null
    assert "docker run -v ${PWD}/test:/certs --name=flannel-e2e-test-flannel1 -d --privileged $FLANNEL_DOCKER_IMAGE --etcd-cafile=/certs/ca.pem --etcd-certfile=/certs/client.pem --etcd-keyfile=/certs/client-key.pem --etcd-endpoints=$etcd_endpt -v 10"

    # rm any old flannel container that maybe running, ignore error as it might not exist
    docker rm -f flannel-e2e-test-flannel2 >/dev/null 2>/dev/null
    assert "docker run -v ${PWD}/test:/certs --name=flannel-e2e-test-flannel2 -d --privileged $FLANNEL_DOCKER_IMAGE --etcd-cafile=/certs/ca.pem --etcd-certfile=/certs/client.pem --etcd-keyfile=/certs/client-key.pem --etcd-endpoints=$etcd_endpt -v 10"
}

teardown() {
    echo "dumping subnets in etcd"
    docker run --rm -e ETCDCTL_API=3 -v "${PWD}/test:/certs" $ETCDCTL_IMG etcdctl --endpoints=$etcd_endpt --cacert=/certs/ca.pem --cert=/certs/client.pem --key=/certs/client-key.pem get --prefix /coreos.com/network/subnets 2>&1
    echo "########## logs for flannel-e2e-test-flannel1 container ##########" 2>&1
    docker logs flannel-e2e-test-flannel1
    echo "########## logs for flannel-e2e-test-flannel2 container ##########" 2>&1
    docker logs flannel-e2e-test-flannel2
    docker rm -f flannel-e2e-test-flannel1 flannel-e2e-test-flannel2 flannel-e2e-test-flannel1-iperf flannel-host1 flannel-host2 > /dev/null 2>&1
    docker run --rm -e ETCDCTL_API=3 -v "${PWD}/test:/certs" $ETCDCTL_IMG etcdctl --endpoints=$etcd_endpt --cacert=/certs/ca.pem --cert=/certs/client.pem --key=/certs/client-key.pem del /coreos.com/network/config > /dev/null 2>&1
}

write_config_etcd() {
    local backend=$1
    if [ -e "$backend" ]; then
        echo "Reading custom conf from $backend"
        flannel_conf=`cat "$backend"`
    else
        flannel_conf="{ \"Network\": \"$FLANNEL_NET\", \"Backend\": { \"Type\": \"${backend}\" } }"
    fi

    while ! docker run --rm -e ETCDCTL_API=3 -v "${PWD}/test:/certs" $ETCDCTL_IMG etcdctl --endpoints=$etcd_endpt --cacert=/certs/ca.pem --cert=/certs/client.pem --key=/certs/client-key.pem put /coreos.com/network/config "$flannel_conf" >/dev/null
    do
        sleep 0.1
    done
}

create_ping_dest() {
    # add a dummy interface with $FLANNEL_SUBNET so we have a known working IP to ping
    for host_num in 1 2; do
       while ! docker exec flannel-e2e-test-flannel$host_num ls /run/flannel/subnet.env >/dev/null 2>&1; do
         sleep 0.1
       done

       # Use declare to allow the host_num variable to be part of the ping_dest variable name. -g is needed to make it global
       declare -g ping_dest$host_num=$(docker "exec" --privileged flannel-e2e-test-flannel$host_num /bin/sh -c '\
        source /run/flannel/subnet.env && \
        ip link add name dummy0 type dummy && \
        ip addr add $FLANNEL_SUBNET dev dummy0 && ip link set dummy0 up && \
        echo $FLANNEL_SUBNET | cut -f 1 -d "/" ')
    done
}

#test_wireguard_ping() {
#    write_config_etcd extension-wireguard
#    create_ping_dest # creates ping_dest1 and ping_dest2 variables
#    pings
#}

test_vxlan_ping() {
    write_config_etcd vxlan
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
}

if [[ ${ARCH} == "amd64" ]]; then
test_udp_ping() {
    write_config_etcd udp
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
}
fi

test_hostgw_ping() {
    write_config_etcd host-gw
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
}

test_ipip_ping() {
    write_config_etcd ipip
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
}

test_ipsec_ping() {
    write_config_etcd ipsec
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
}

test_wireguard_ping() {
    write_config_etcd wireguard
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
}

pings() {
    # ping in both directions
    assert "docker exec --privileged flannel-e2e-test-flannel1 /bin/ping -I $ping_dest1 -c 3 $ping_dest2" "Host 1 cannot ping host 2"
    assert "docker exec --privileged flannel-e2e-test-flannel2 /bin/ping -I $ping_dest2 -c 3 $ping_dest1" "Host 2 cannot ping host 1"
}

test_ipsec_perf() {
    write_config_etcd ipsec
    create_ping_dest
    perf
}

perf() {
    # Perf test - run iperf server on flannel1 and client on flannel2
    docker rm -f flannel-e2e-test-flannel1-iperf 2>/dev/null
    docker run -d --name flannel-e2e-test-flannel1-iperf --net=container:flannel-e2e-test-flannel1 iperf3:latest >/dev/null
    wait_for flannel-e2e-test-flannel1-iperf
    docker run --rm --net=container:flannel-e2e-test-flannel2 iperf3:latest -c $ping_dest1 -B $ping_dest2
}

wait_for() {
  while ! docker inspect --format='{{json .State.Status}}' $1 >/dev/null
  do
    sleep 1
  done
}


test_multi() {
    flannel_conf_vxlan='{"Network": "10.11.0.0/16", "Backend": {"Type": "vxlan"}}'
    flannel_conf_host_gw='{"Network": "10.12.0.0/16", "Backend": {"Type": "host-gw"}}'

    while ! docker run --rm -e ETCDCTL_API=3 -v "${PWD}/test:/certs" $ETCD_IMG etcdctl --endpoints=$etcd_endpt --cacert=/certs/ca.pem --cert=/certs/client.pem --key=/certs/client-key.pem put /vxlan/network/config "$flannel_conf_vxlan" >/dev/null
    do
        sleep 0.1
    done

    while ! docker run --rm -e ETCDCTL_API=3 -v "${PWD}/test:/certs" $ETCD_IMG etcdctl --endpoints=$etcd_endpt --cacert=/certs/ca.pem --cert=/certs/client.pem --key=/certs/client-key.pem put /hostgw/network/config "$flannel_conf_host_gw" >/dev/null
    do
        sleep 0.1
    done

    for host in 1 2; do
        # rm any old flannel container, ignore error as it might not exist
        docker rm -f flannel-host$host 2>/dev/null >/dev/null

        # Start the hosts
        docker run -v "${PWD}/test:/certs" --name=flannel-host$host -id --privileged --entrypoint /bin/sh $FLANNEL_DOCKER_IMAGE   >/dev/null

        # Start two flanneld instances
        docker exec -d flannel-host$host sh -c "/opt/bin/flanneld -v 10 -subnet-file /vxlan.env -etcd-prefix=/vxlan/network --etcd-cafile=/certs/ca.pem --etcd-certfile=/certs/client.pem --etcd-keyfile=/certs/client-key.pem --etcd-endpoints=$etcd_endpt 2>vxlan.log"
        docker exec -d flannel-host$host sh -c "/opt/bin/flanneld -v 10 -subnet-file /hostgw.env -etcd-prefix=/hostgw/network --etcd-cafile=/certs/ca.pem --etcd-certfile=/certs/client.pem --etcd-keyfile=/certs/client-key.pem --etcd-endpoints=$etcd_endpt 2>hostgw.log"
    done

    for host in 1 2; do
        for backend_type in vxlan hostgw; do
            while ! docker exec flannel-host$host ls /$backend_type.env  >/dev/null 2>&1; do
              sleep 0.1
            done
        done
    done

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
    assert "docker exec flannel-host2 ping -c 3 $hostgw_ping_dest"
    assert "docker exec flannel-host2 ping -c 3 $vxlan_ping_dest"
    assert_fails "docker exec flannel-host2 ping -W 1 -c 1 -I flannel.1 $hostgw_ping_dest"
    assert_fails "docker exec flannel-host2 ping -W 1 -c 1 -I eth0 $vxlan_ping_dest"
}
