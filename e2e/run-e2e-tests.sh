#!/bin/bash

set -e -o pipefail

source $(dirname $0)/version.sh
source $(dirname $0)/e2e-functions.sh

FLANNEL_NET="${FLANNEL_NET:-10.42.0.0/16}"
FLANNEL_IP6NET="${FLANNEL_IP6NET:-2001:cafe:42:0::/56}"
# needs to be exported for yq
export FLANNEL_IMAGE="quay.io/coreos/flannel:${TAG}-${ARCH}"

KIND_CLUSTER_NAME="e2e"
KIND_CONTROL_PLANE="${KIND_CLUSTER_NAME}-control-plane"
KIND_WORKER="${KIND_CLUSTER_NAME}-worker"

setup_suite() {
    $(dirname $0)/download-kubectl.sh
    install_cni_plugins
}

install_cni_plugins() {
    local arch="${ARCH:-amd64}"
    local cni_version="v1.9.1"
    local cni_tgz="cni-plugins-linux-${arch}-${cni_version}.tgz"
    local cni_url="https://github.com/containernetworking/plugins/releases/download/${cni_version}/${cni_tgz}"

    case "${arch}" in
        amd64)  cni_sha256="b98f74a0f8522f0a83867178729c1aa70f2158f90c45a2ca8fa791db1c76b303" ;;
        arm)    cni_sha256="21416880bea0541d78afaf106373d6dbb471edb92c0114fa263494fe4aec8d3b" ;;
        arm64)  cni_sha256="56171987d3947707c3563db2f4001bccaf50fd63468611b9f3cbecb1375ee7ec" ;;
        *) echo "unsupported ARCH for CNI plugins: ${arch}" >&2; return 1 ;;
    esac

    echo "Installing CNI plugins ${cni_version} for ${arch}..."
    curl -fsSLo "/tmp/${cni_tgz}" "${cni_url}"
    echo "${cni_sha256}  /tmp/${cni_tgz}" | sha256sum --check --status
    sudo mkdir -p /opt/cni/bin
    sudo tar -C /opt/cni/bin -xzf "/tmp/${cni_tgz}"
    rm -f "/tmp/${cni_tgz}"
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

create_iperf_server_pod() {
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
    image: iperf3:latest
    imagePullPolicy: IfNotPresent
  nodeName: ${worker_node}
EOF
}

create_iperf_client_pod() {
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
    image: iperf3:latest
    imagePullPolicy: IfNotPresent
    command:
    - /bin/sh
    - -c
    - while true; do sleep 3600; done
  nodeName: ${worker_node}
EOF
}

write-flannel-conf(){
    local backend=$1
    local enable_nftables=$2
    cp ../Documentation/kube-flannel.yml ./kube-flannel.yml
    yq -i 'select(.kind == "DaemonSet").spec.template.spec.containers[0].image |= strenv(FLANNEL_IMAGE)' ./kube-flannel.yml
    yq -i 'select(.kind == "DaemonSet").spec.template.spec.initContainers[1].image |= strenv(FLANNEL_IMAGE)' ./kube-flannel.yml

    export flannel_conf="{ \"Network\": \"$FLANNEL_NET\", \"Backend\": { \"Type\": \"${backend}\" }, \"EnableNFTables\": ${enable_nftables} }"

    yq -i 'select(.metadata.name == "kube-flannel-cfg").data."net-conf.json" |= strenv(flannel_conf)' ./kube-flannel.yml

    # udp backend needs to run in "privileged" mode to access /dev/net/tun
    if [ "$backend" = "udp" ]; then
        yq -i 'select(.kind == "DaemonSet").spec.template.spec.containers[0].securityContext.privileged |= true'  kube-flannel.yml
    fi
}

# This is not used at the moment since github runners don't support dual-stack networking
write-flannel-conf-dual-stack(){
    local backend=$1
    local enable_nftables=$2
    cp ../Documentation/kube-flannel.yml ./kube-flannel.yml
    yq -i 'select(.kind == "DaemonSet").spec.template.spec.containers[0].image |= strenv(FLANNEL_IMAGE)' ./kube-flannel.yml

    export flannel_conf="{ \"EnableIPv6\": true, \"Network\": \"$FLANNEL_NET\", \"IPv6Network\":\"${FLANNEL_IP6NET}\", \"Backend\": { \"Type\": \"${backend}\" }, \"EnableNFTables\": ${enable_nftables} }"

    yq -i 'select(.metadata.name == "kube-flannel-cfg").data."net-conf.json" |= strenv(flannel_conf)' ./kube-flannel.yml
}

install-flannel() {
    kubectl --kubeconfig="${HOME}/.kube/config" apply -f ./kube-flannel.yml
}

delete-flannel() {
    kubectl --kubeconfig="${HOME}/.kube/config" delete -f ./kube-flannel.yml
}

get_pod_ip() {
    local pod_name=$1
    kubectl --kubeconfig="${HOME}/.kube/config" get pod ${pod_name} --template '{{.status.podIP}}'
}

get_pod_cidr() {
    local node_name=$1
    kubectl --kubeconfig="${HOME}/.kube/config" get node ${node_name} --template '{{.spec.podCIDR}}'
}

get_pod_logs() {
    local pod_name=$1
    kubectl --kubeconfig="${HOME}/.kube/config" logs $pod_name -n kube-flannel
}

dump_debug_info() {
    echo "======== DEBUG INFO ========"
    echo "--- nodes ---"
    kubectl --kubeconfig="${HOME}/.kube/config" get nodes -o wide
    echo "--- all pods ---"
    kubectl --kubeconfig="${HOME}/.kube/config" get pods -A -o wide
    echo "--- flannel pods describe ---"
    kubectl --kubeconfig="${HOME}/.kube/config" describe pods -n kube-flannel 2>/dev/null || true
    echo "--- flannel pod logs (all nodes) ---"
    kubectl --kubeconfig="${HOME}/.kube/config" get pods -n kube-flannel -o wide 2>/dev/null | tail -n +2 | awk '{print $1}' | while read pod; do
        echo "--- flannel pod $pod ---"
        kubectl --kubeconfig="${HOME}/.kube/config" logs "$pod" -n kube-flannel --all-containers --previous 2>/dev/null && echo "(previous)" || true
        kubectl --kubeconfig="${HOME}/.kube/config" logs "$pod" -n kube-flannel --all-containers 2>/dev/null || true
    done
    echo "--- flannel files on kind nodes ---"
    for node in "${KIND_CONTROL_PLANE}" "${KIND_WORKER}"; do
        echo "--- ${node}:/run/flannel ---"
        docker exec "${node}" ls -al /run/flannel 2>/dev/null || true
        docker exec "${node}" cat /run/flannel/subnet.env 2>/dev/null || true
    done
    echo "--- flannel-related images on kind nodes ---"
    for node in "${KIND_CONTROL_PLANE}" "${KIND_WORKER}"; do
        echo "--- ${node} images ---"
        docker exec "${node}" crictl images 2>/dev/null | grep -E 'quay.io/coreos/flannel|iperf3' || true
    done
    echo "--- events (sorted) ---"
    kubectl --kubeconfig="${HOME}/.kube/config" get events --sort-by='.lastTimestamp' -A
    echo "======== END DEBUG INFO ========"
}

wait_for_subnet_env() {
    local timeout_seconds="${SUBNET_ENV_TIMEOUT_SECONDS:-120}"
    local poll_interval=2
    local node

    echo "waiting for /run/flannel/subnet.env on all nodes..."
    for node in "${KIND_CONTROL_PLANE}" "${KIND_WORKER}"; do
        local waited=0
        while ! docker exec "${node}" test -f /run/flannel/subnet.env 2>/dev/null; do
            if [ "${waited}" -ge "${timeout_seconds}" ]; then
                echo "timed out waiting for /run/flannel/subnet.env on ${node}"
                docker exec "${node}" ls -al /run/flannel 2>/dev/null || true
                return 1
            fi
            echo "  subnet.env not yet present on ${node}, waiting..."
            sleep "${poll_interval}"
            waited=$((waited + poll_interval))
        done
        echo "  subnet.env present on ${node}"
        docker exec "${node}" cat /run/flannel/subnet.env
    done
}

pings() {
    create_test_pod multitool1 ${KIND_WORKER}
    create_test_pod multitool2 ${KIND_CONTROL_PLANE}

    # wait for test-pods to be ready
    echo "wait for test-pods to be ready..."
    timeout --foreground 1m bash -c "e2e-wait-for-test-pods"
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "test pods not ready in time. Checking their status..."
        dump_debug_info
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
    create_iperf_server_pod iperf3-server ${KIND_WORKER}
    create_iperf_client_pod iperf3-client ${KIND_CONTROL_PLANE}

    # wait for test-pods to be ready
    echo "wait for test-pods to be ready..."
    WAIT_FOR_PODS="iperf3-server iperf3-client" timeout --foreground 1m bash -c "e2e-wait-for-test-pods"

    ip_1=$(get_pod_ip iperf3-server)

    echo "starting iperf3 server..." >&2
    sleep 5
    echo "starting iperf3 client..." >&2
    assert "kubectl --kubeconfig="${HOME}/.kube/config" exec iperf3-client -- iperf3 -c ${ip_1} -t 10"
}

prepare_test() {
    local backend=$1
    local enable_nftables=${2:-false}
    # install flannel version to test
    write-flannel-conf ${backend} ${enable_nftables}
    
    install-flannel
    
    # wait for the flannel DaemonSet to fully roll out before anything else.
    # kind nodes are already Ready before flannel is installed, so e2e-wait-for-nodes
    # would return immediately. We must ensure /run/flannel/subnet.env exists on every
    # node before CoreDNS tries to use the flannel CNI plugin.
    echo "waiting for flannel DaemonSet to roll out..."
    if ! kubectl --kubeconfig="${HOME}/.kube/config" rollout status daemonset/kube-flannel-ds \
        -n kube-flannel --timeout=5m; then
        echo "flannel DaemonSet did not roll out in time. Checking cluster state..."
        dump_debug_info
        exit 1
    fi

    # rollout status considers a pod "available" as soon as its container is Running,
    # but flannel writes subnet.env slightly after startup. Poll each node directly.
    if ! wait_for_subnet_env; then
        echo "/run/flannel/subnet.env was not created on all nodes in time. Checking cluster state..."
        dump_debug_info
        exit 1
    fi
    
    # wait for nodes to be ready
    timeout --foreground 5m bash -c "e2e-wait-for-nodes"
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "test nodes not ready in time. Checking their status..."
        dump_debug_info
        exit $retVal
    fi
    # wait for services to be ready
    echo "wait for services to be ready..."
    timeout --foreground 5m bash -c "e2e-wait-for-services"
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "services not ready in time. Checking their status..."
        dump_debug_info
        exit $retVal
    fi
}

setup() {
    echo "Creating kind cluster..."
    kind create cluster --name ${KIND_CLUSTER_NAME} --config $(dirname $0)/kind-config.yaml
    echo "Loading flannel image into kind cluster..."
    kind load docker-image ${FLANNEL_IMAGE} --name ${KIND_CLUSTER_NAME}
    echo "Loading iperf3 image into kind cluster..."
    kind load docker-image iperf3:latest --name ${KIND_CLUSTER_NAME}
    mkdir -p "${HOME}/.kube"
    kind export kubeconfig --name ${KIND_CLUSTER_NAME} --kubeconfig "${HOME}/.kube/config"
    echo "kubeconfig is at ${HOME}/.kube/config"
}
test_vxlan() {
    prepare_test vxlan
    pings
    check_iptables
    delete-flannel
}

test_vxlan_nft() {
    prepare_test vxlan true
    pings
    check_nftables
    delete-flannel
}

test_wireguard() {
    prepare_test wireguard
    pings
    check_iptables
    delete-flannel
}

test_host-gw() {
    prepare_test host-gw
    pings
    check_iptables
    delete-flannel
}

if [[ ${ARCH} == "amd64" ]]; then
test_udp() {
    prepare_test udp
    pings
    check_iptables
    delete-flannel
}
fi

test_ipip() {
    prepare_test ipip
    pings
    check_iptables
    delete-flannel
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
    kind delete cluster --name ${KIND_CLUSTER_NAME}
}

check_iptables() {
  local worker_podcidr=$(get_pod_cidr ${KIND_WORKER})
  local leader_pod_cidr=$(get_pod_cidr ${KIND_CONTROL_PLANE})
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
-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes load balancer firewall" -j KUBE-PROXY-FIREWALL
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
                "$(docker exec --privileged ${KIND_WORKER} /usr/sbin/iptables -t nat -S POSTROUTING | grep FLANNEL)
$(docker exec --privileged ${KIND_WORKER} /usr/sbin/iptables -t nat -S FLANNEL-POSTRTG)" "Host 1 has not expected postrouting rules"
  assert_equals "$POSTROUTING_RULES_LEADER" \
                "$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/iptables -t nat -S POSTROUTING | grep FLANNEL)
$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/iptables -t nat -S FLANNEL-POSTRTG)" "Host 2 has not expected postrouting rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged ${KIND_WORKER} /usr/sbin/iptables -t filter -S FORWARD)
$(docker exec --privileged ${KIND_WORKER} /usr/sbin/iptables -t filter -S FLANNEL-FWD -w 5)" "Host 1 has not expected forward rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/iptables -t filter -S FORWARD)
$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/iptables -t filter -S FLANNEL-FWD)" "Host 2 has not expected forward rules"
}

check_iptables_removed() {
  local worker_podcidr=$(get_pod_cidr ${KIND_WORKER})
  local leader_pod_cidr=$(get_pod_cidr ${KIND_CONTROL_PLANE})
  read -r -d '' POSTROUTING_RULES_WORKER << EOM
-N FLANNEL-POSTRTG
EOM
  read -r -d '' POSTROUTING_RULES_LEADER << EOM
-N FLANNEL-POSTRTG
EOM
  read -r -d '' FORWARD_RULES << EOM
-P FORWARD ACCEPT
-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes load balancer firewall" -j KUBE-PROXY-FIREWALL
-A FORWARD -m comment --comment "kubernetes forwarding rules" -j KUBE-FORWARD
-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A FORWARD -m conntrack --ctstate NEW -m comment --comment "kubernetes externally-visible service portals" -j KUBE-EXTERNAL-SERVICES
-N FLANNEL-FWD
EOM
# check that masquerade & forward rules have been removed
  assert_equals "$POSTROUTING_RULES_WORKER" \
                "$(docker exec --privileged ${KIND_WORKER} /usr/sbin/iptables -t nat -S POSTROUTING | grep FLANNEL)$(docker exec --privileged ${KIND_WORKER} /usr/sbin/iptables -t nat -S FLANNEL-POSTRTG)" "Host 1 has not expected postrouting rules"
  assert_equals "$POSTROUTING_RULES_LEADER" \
                "$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/iptables -t nat -S POSTROUTING | grep FLANNEL)$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/iptables -t nat -S FLANNEL-POSTRTG)" "Host 2 has not expected postrouting rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged ${KIND_WORKER} /usr/sbin/iptables -t filter -S FORWARD)
$(docker exec --privileged ${KIND_WORKER} /usr/sbin/iptables -t filter -S FLANNEL-FWD -w 5)" "Host 1 has not expected forward rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/iptables -t filter -S FORWARD)
$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/iptables -t filter -S FLANNEL-FWD)" "Host 2 has not expected forward rules"
}

###nftables
check_nftables() {
  local worker_podcidr=$(get_pod_cidr ${KIND_WORKER})
  local leader_podcidr=$(get_pod_cidr ${KIND_CONTROL_PLANE})
  read -d '' POSTROUTING_RULES_WORKER << EOM
table ip flannel-ipv4 {
	chain postrtg {
		comment "chain to manage traffic masquerading by flannel"
		type nat hook postrouting priority srcnat; policy accept;
		meta mark 0x00004000 return
		ip saddr ${worker_podcidr} ip daddr 10.42.0.0/16 return
		ip saddr 10.42.0.0/16 ip daddr ${worker_podcidr} return
		ip saddr != ${worker_podcidr} ip daddr 10.42.0.0/16 return
		ip saddr 10.42.0.0/16 ip daddr != 224.0.0.0/4 masquerade fully-random
		ip saddr != 10.42.0.0/16 ip daddr 10.42.0.0/16 masquerade fully-random
	}
}
EOM
  read -r -d '' POSTROUTING_RULES_LEADER << EOM
table ip flannel-ipv4 {
	chain postrtg {
		comment "chain to manage traffic masquerading by flannel"
		type nat hook postrouting priority srcnat; policy accept;
		meta mark 0x00004000 return
		ip saddr ${leader_podcidr} ip daddr 10.42.0.0/16 return
		ip saddr 10.42.0.0/16 ip daddr ${leader_podcidr} return
		ip saddr != ${leader_podcidr} ip daddr 10.42.0.0/16 return
		ip saddr 10.42.0.0/16 ip daddr != 224.0.0.0/4 masquerade fully-random
		ip saddr != 10.42.0.0/16 ip daddr 10.42.0.0/16 masquerade fully-random
	}
}
EOM
  read -r -d '' FORWARD_RULES << EOM
table ip flannel-ipv4 {
	chain forward {
		comment "chain to accept flannel traffic"
		type filter hook forward priority filter; policy accept;
		ip saddr 10.42.0.0/16 accept
		ip daddr 10.42.0.0/16 accept
	}
}
EOM
  # check masquerade & forward rules
  assert_equals "$POSTROUTING_RULES_WORKER" \
                "$(docker exec --privileged ${KIND_WORKER} /usr/sbin/nft list chain flannel-ipv4 postrtg)" "Node worker does not have expected postrouting rules"
  assert_equals "$POSTROUTING_RULES_LEADER" \
                "$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/nft list chain flannel-ipv4 postrtg)" "Node leader does not have expected postrouting rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged ${KIND_WORKER} /usr/sbin/nft list chain flannel-ipv4 forward)" "Node worker does not have expected forward rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/nft list chain flannel-ipv4 forward)" "Node leader does not have expected forward rules"
}

check_nftables_removed() {
  # check masquerade & forward rules
  assert_equals "" \
                "$(docker exec --privileged ${KIND_WORKER} /usr/sbin/nft list chain flannel-ipv4 postrtg)" "Node worker has unexpected postrouting rules"
  assert_equals "" \
                "$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/nft list chain flannel-ipv4 postrtg)" "Node leader has unexpected postrouting rules"
  assert_equals "" \
                "$(docker exec --privileged ${KIND_WORKER} /usr/sbin/nft list chain flannel-ipv4 forward)" "Node worker has unexpected forward rules"
  assert_equals "" \
                "$(docker exec --privileged ${KIND_CONTROL_PLANE} /usr/sbin/nft list chain flannel-ipv4 forward)" "Node leader has unexpected forward rules"
}