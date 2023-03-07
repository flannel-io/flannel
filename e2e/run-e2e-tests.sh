#!/bin/bash

set -e -o pipefail

source $(dirname $0)/version.sh
source $(dirname $0)/e2e-functions.sh

FLANNEL_NET="${FLANNEL_NET:-10.42.0.0/16}"
FLANNEL_IP6NET="${FLANNEL_IP6NET:-2001:cafe:42:0::/56}"
# needs to be exported for yq
export FLANNEL_IMAGE="quay.io/coreos/flannel:${TAG}-${ARCH}"


setup_suite() {
    # copy flannel image built by `make image` to docker compose context folder
    rm -rf $(dirname $0)/scratch
    mkdir -p $(dirname $0)/scratch
    cp $(dirname $0)/../dist/${FLANNEL_IMAGE_FILE}.docker $(dirname $0)/scratch/${FLANNEL_IMAGE_FILE}.tar

    $(dirname $0)/download-kubectl.sh
}

create_test_pod() {
    local pod_name=$1
    local worker_node=$2
    cat <<EOF | kubectl --kubeconfig="${HOME}/.kube/config" apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
spec:
  containers:
  - name: ${pod_name}
    image: wbitt/network-multitool:alpine-extra
  nodeName: ${worker_node}
EOF
}

write-flannel-conf(){
    local backend=$1
    cp ../Documentation/kube-flannel.yml ./kube-flannel.yml
    yq -i 'select(.kind == "DaemonSet").spec.template.spec.containers[0].image |= strenv(FLANNEL_IMAGE)' ./kube-flannel.yml
    yq -i 'select(.kind == "DaemonSet").spec.template.spec.initContainers[1].image |= strenv(FLANNEL_IMAGE)' ./kube-flannel.yml

    export flannel_conf="{ \"Network\": \"$FLANNEL_NET\", \"Backend\": { \"Type\": \"${backend}\" } }"

    yq -i 'select(.metadata.name == "kube-flannel-cfg").data."net-conf.json" |= strenv(flannel_conf)' ./kube-flannel.yml

    # udp backend needs to run in "privileged" mode to access /dev/net/tun
    if [ "$backend" = "udp" ]; then
        yq -i 'select(.kind == "DaemonSet").spec.template.spec.containers[0].securityContext.privileged |= true'  kube-flannel.yml
    fi
}

# This is not used at the moment since github runners don't support dual-stack networking
write-flannel-conf-dual-stack(){
    local backend=$1
    cp ../Documentation/kube-flannel.yml ./kube-flannel.yml
    yq -i 'select(.kind == "DaemonSet").spec.template.spec.containers[0].image |= strenv(FLANNEL_IMAGE)' ./kube-flannel.yml

    export flannel_conf="{ \"EnableIPv6\": true, \"Network\": \"$FLANNEL_NET\", \"IPv6Network\":\"${FLANNEL_IP6NET}\", \"Backend\": { \"Type\": \"${backend}\" } }"

    yq -i 'select(.metadata.name == "kube-flannel-cfg").data."net-conf.json" |= strenv(flannel_conf)' ./kube-flannel.yml
}

install-flannel() {
    kubectl --kubeconfig="${HOME}/.kube/config" apply -f ./kube-flannel.yml
}

get_pod_ip() {
    local pod_name=$1
    kubectl --kubeconfig="${HOME}/.kube/config" get pod ${pod_name} --template '{{.status.podIP}}'
}

get_pod_cidr() {
    local node_name=$1
    kubectl --kubeconfig="${HOME}/.kube/config" get node ${node_name} --template '{{.spec.podCIDR}}'
}

# get_flannel_logs() 

pings() {
    create_test_pod multitool1 local-worker
    create_test_pod multitool2 local-leader

    # wait for test-pods to be ready
    echo "wait for test-pods to be ready..."
    timeout --foreground 1m bash -c "e2e-wait-for-test-pods"
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "test pods not ready in time. Checking their status..."
        kubectl --kubeconfig="${HOME}/.kube/config" get events --sort-by='.lastTimestamp' -A
        exit $retVal
    fi
    
    ip_1=$(get_pod_ip multitool1)
    ip_2=$(get_pod_ip multitool2)

    echo "multitool1 IP is: ${ip_1}"
    echo "multitool2 IP is: ${ip_2}"

    timeout --foreground 1m bash -c "e2e-wait-for-ping multitool1 ${ip_2}"

    assert "kubectl --kubeconfig="${HOME}/.kube/config" exec multitool1 -- ping -c 5 ${ip_2}"
    assert "kubectl --kubeconfig="${HOME}/.kube/config" exec multitool2 -- ping -c 5 ${ip_1}"
}

perf() {
    create_test_pod multitool1 local-worker
    create_test_pod multitool2 local-leader

    # wait for test-pods to be ready
    echo "wait for test-pods to be ready..."
    timeout --foreground 1m bash -c "e2e-wait-for-test-pods"

    ip_1=$(get_pod_ip multitool1)
    ip_2=$(get_pod_ip multitool2)

    echo "starting iperf3 server..." >&2
    kubectl --kubeconfig="${HOME}/.kube/config" exec multitool1 -- iperf3 -s &
    sleep 5
    echo "starting iperf3 client..." >&2
    assert "kubectl --kubeconfig="${HOME}/.kube/config" exec multitool2 -- iperf3 -c ${ip_1} -t 10"
}

prepare_test() {
    local backend=$1
    # install flannel version to test
    write-flannel-conf ${backend} 
    
    install-flannel
    # wait for nodes to be ready
    timeout --foreground 5m bash -c "e2e-wait-for-nodes"
    # wait for services to be ready
    echo "wait for services to be ready..."
    timeout --foreground 5m bash -c "e2e-wait-for-services"
}

setup() {
    # start k3s cluster
    echo "starting k3s cluster..."
    docker compose up -d
    # get kubeconfig for the k3s cluster
    echo "waiting for kubeconfig to be available..."
    ./get-kubeconfig.sh
    echo "kubeconfig is at "${HOME}/.kube/config":"
}

test_vxlan() {
    prepare_test vxlan
    pings
    check_iptables
}

test_wireguard() {
    prepare_test wireguard
    pings
    check_iptables
}

test_host-gw() {
    prepare_test host-gw
    pings
    check_iptables
}

if [[ ${ARCH} == "amd64" ]]; then
test_udp() {
    prepare_test udp
    pings
    check_iptables
}
fi

test_ipip() {
    prepare_test ipip
    pings
    check_iptables
}

test_perf_vxlan() {
    prepare_test vxlan
    perf
}

test_perf_wireguard() {
    prepare_test wireguard
    perf
}

test_perf_host-gw() {
    prepare_test host-gw
    perf
}

test_perf_ipip() {
    prepare_test ipip
    perf
}

if [[ ${ARCH} == "amd64" ]]; then
    test_perf_udp() {
        prepare_test udp
        perf
    }
fi

teardown() {
    docker compose down   \
        --remove-orphans \
        --rmi local \
        --volumes
}

check_iptables() {
  local worker_podcidr=$(get_pod_cidr local-worker)
  local leader_pod_cidr=$(get_pod_cidr local-leader)
  read -r -d '' POSTROUTING_RULES_WORKER << EOM
-A POSTROUTING -m comment --comment "flanneld masq" -j FLANNEL-POSTRTG
-N FLANNEL-POSTRTG
-A FLANNEL-POSTRTG -m mark --mark 0x4000/0x4000 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s ${worker_podcidr} -d 10.42.0.0/16 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s 10.42.0.0/16 -d ${worker_podcidr} -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG ! -s 10.42.0.0/16 -d ${worker_podcidr} -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s 10.42.0.0/16 ! -d 224.0.0.0/4 -m comment --comment "flanneld masq" -j MASQUERADE --random-fully
-A FLANNEL-POSTRTG ! -s 10.42.0.0/16 -d 10.42.0.0/16 -m comment --comment "flanneld masq" -j MASQUERADE --random-fully
EOM
  read -r -d '' POSTROUTING_RULES_LEADER << EOM
-A POSTROUTING -m comment --comment "flanneld masq" -j FLANNEL-POSTRTG
-N FLANNEL-POSTRTG
-A FLANNEL-POSTRTG -m mark --mark 0x4000/0x4000 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s ${leader_pod_cidr} -d 10.42.0.0/16 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s 10.42.0.0/16 -d ${leader_pod_cidr} -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG ! -s 10.42.0.0/16 -d ${leader_pod_cidr} -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s 10.42.0.0/16 ! -d 224.0.0.0/4 -m comment --comment "flanneld masq" -j MASQUERADE --random-fully
-A FLANNEL-POSTRTG ! -s 10.42.0.0/16 -d 10.42.0.0/16 -m comment --comment "flanneld masq" -j MASQUERADE --random-fully
EOM
  read -r -d '' FORWARD_RULES << EOM
-P FORWARD ACCEPT
-A FORWARD -m comment --comment "kubernetes forwarding rules" -j KUBE-FORWARD
-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes externally-visible service portals" -j KUBE-EXTERNAL-SERVICES
-A FORWARD -m comment --comment "flanneld forward" -j FLANNEL-FWD
-N FLANNEL-FWD
-A FLANNEL-FWD -s 10.42.0.0/16 -m comment --comment "flanneld forward" -j ACCEPT
-A FLANNEL-FWD -d 10.42.0.0/16 -m comment --comment "flanneld forward" -j ACCEPT
EOM
  # check masquerade & forward rules
  assert_equals "$POSTROUTING_RULES_WORKER" \
                "$(docker exec --privileged local-worker /usr/sbin/iptables -t nat -S POSTROUTING | grep FLANNEL)
$(docker exec --privileged local-worker /usr/sbin/iptables -t nat -S FLANNEL-POSTRTG)" "Host 1 has not expected postrouting rules"
  assert_equals "$POSTROUTING_RULES_LEADER" \
                "$(docker exec --privileged local-leader /usr/sbin/iptables -t nat -S POSTROUTING | grep FLANNEL)
$(docker exec --privileged local-leader /usr/sbin/iptables -t nat -S FLANNEL-POSTRTG)" "Host 2 has not expected postrouting rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged local-worker /usr/sbin/iptables -t filter -S FORWARD)
$(docker exec --privileged local-worker /usr/sbin/iptables -t filter -S FLANNEL-FWD -w 5)" "Host 1 has not expected forward rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged local-leader /usr/sbin/iptables -t filter -S FORWARD)
$(docker exec --privileged local-leader /usr/sbin/iptables -t filter -S FLANNEL-FWD)" "Host 2 has not expected forward rules"
}
