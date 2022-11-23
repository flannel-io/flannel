#!/bin/bash

ARCH="${ARCH:-amd64}"
ETCD_IMG="${ETCD_IMG:-quay.io/coreos/etcd:v3.5.1}"
ETCD_LOCATION="${ETCD_LOCATION:-etcd}"
FLANNEL_NET="${FLANNEL_NET:-10.10.0.0/16}"
TAG=`git describe --tags --dirty`
FLANNEL_DOCKER_IMAGE="${FLANNEL_DOCKER_IMAGE:-quay.io/coreos/flannel:$TAG}"
K8S_VERSION="${K8S_VERSION:-1.25.2}"
HYPERKUBE_IMG="docker.io/rancher/hyperkube"
HYPERKUBE_CMD="${HYPERKUBE_CMD:-" "}"
HYPERKUBE_APISERVER_CMD="${HYPERKUBE_APISERVER_CMD:-kube-apiserver}"

docker_ip=$(ip -o -f inet addr show docker0 | grep -Po 'inet \K[\d.]+')
etcd_endpt="http://$docker_ip:2379"
k8s_endpt="https://$docker_ip:6443"

# Set the proper imagename according to architecture
if [[ ${ARCH} == "ppc64le" ]]; then
    ETCD_IMG+="-ppc64le"
elif [[ ${ARCH} == "arm64" ]]; then
    ETCD_IMG+="-arm64"
fi

setup_suite() {
    # Run etcd, killing any existing one that was running

    # Start etcd
    docker rm -f flannel-e2e-test-etcd >/dev/null 2>/dev/null
    docker run --name=flannel-e2e-test-etcd -d -p 2379:2379 -e ETCD_UNSUPPORTED_ARCH=${ARCH} $ETCD_IMG etcd --listen-client-urls http://0.0.0.0:2379 --advertise-client-urls $etcd_endpt >/dev/null
    sleep 1

    # Start a kubernetes API server
    docker rm -f flannel-e2e-k8s-apiserver >/dev/null 2>/dev/null
    dir=$(mktemp -d)
    
    mkdir $dir/pki
    
    openssl genrsa -out $dir/pki/ca.key 2048
    openssl req -new -key $dir/pki/ca.key -subj "/CN=KUBERNETES-CA/O=Kubernetes" -out $dir/pki/ca.csr
    openssl x509 -req -in $dir/pki/ca.csr -signkey $dir/pki/ca.key -CAcreateserial  -out $dir/pki/ca.crt -days 1000
    cat > $dir/openssl.cnf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_req]
basicConstraints = critical, CA:FALSE
keyUsage = critical, nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = kubernetes
DNS.2 = kubernetes.default
DNS.3 = kubernetes.default.svc
DNS.4 = kubernetes.default.svc.cluster
DNS.5 = kubernetes.default.svc.cluster.local
IP.1 = $docker_ip
IP.2 = 127.0.0.1
EOF
    openssl genrsa -out $dir/pki/kube-apiserver.key 2048
    openssl req -new -key $dir/pki/kube-apiserver.key \
    -subj "/CN=kube-apiserver/O=Kubernetes" -out $dir/pki/kube-apiserver.csr -config $dir/openssl.cnf
    openssl x509 -req -in $dir/pki/kube-apiserver.csr \
    -CA $dir/pki/ca.crt -CAkey $dir/pki/ca.key -CAcreateserial  -out $dir/pki/kube-apiserver.crt -extensions v3_req -extfile $dir/openssl.cnf -days 1000

    openssl genrsa -out $dir/pki/service-account.key 2048
    openssl req -new -key $dir/pki/service-account.key \
    -subj "/CN=service-accounts/O=Kubernetes" -out $dir/pki/service-account.csr
    openssl x509 -req -in $dir/pki/service-account.csr \
    -CA $dir/pki/ca.crt -CAkey $dir/pki/ca.key -CAcreateserial  -out $dir/pki/service-account.crt -days 100

    openssl genrsa -out $dir/pki/admin.key 2048
    openssl req -new -key $dir/pki/admin.key -subj "/CN=admin/O=system:masters" -out $dir/pki/admin.csr
    openssl x509 -req -in $dir/pki/admin.csr -CA $dir/pki/ca.crt -CAkey $dir/pki/ca.key -CAcreateserial  -out $dir/pki/admin.crt -days 1000
    
    docker run -d --net=host -v $dir:/var/lib/kubernetes --name flannel-e2e-k8s-apiserver ${HYPERKUBE_IMG}:v$K8S_VERSION-rancher1-linux-$ARCH \
      ${HYPERKUBE_CMD} ${HYPERKUBE_APISERVER_CMD} --etcd-servers=$etcd_endpt --bind-address=$docker_ip \
      --client-ca-file=/var/lib/kubernetes/pki/ca.crt \
      --enable-admission-plugins=NodeRestriction,ServiceAccount \
      --service-account-key-file=/var/lib/kubernetes/pki/service-account.crt \
      --service-account-signing-key-file=/var/lib/kubernetes/pki/service-account.key \
      --service-account-issuer=https://kubernetes.default.svc.local \
      --tls-cert-file=/var/lib/kubernetes/pki/kube-apiserver.crt \
      --tls-private-key-file=/var/lib/kubernetes/pki/kube-apiserver.key \
      --service-cluster-ip-range=10.101.0.0/16 --allow-privileged >/dev/null
    sleep 1

    docker exec flannel-e2e-k8s-apiserver kubectl config set-cluster kubernetes-test-flannel \
    --certificate-authority=/var/lib/kubernetes/pki/ca.crt \
    --embed-certs=true \
    --server="https://$docker_ip:6443" \
    --kubeconfig=/var/lib/kubernetes/admin.kubeconfig

    docker exec flannel-e2e-k8s-apiserver kubectl config set-credentials admin \
    --client-certificate=/var/lib/kubernetes/pki/admin.crt \
    --client-key=/var/lib/kubernetes/pki/admin.key \
    --embed-certs=true \
    --kubeconfig=/var/lib/kubernetes/admin.kubeconfig

    docker exec flannel-e2e-k8s-apiserver kubectl config set-context default \
    --cluster=kubernetes-test-flannel \
    --user=admin \
    --kubeconfig=/var/lib/kubernetes/admin.kubeconfig

    docker exec flannel-e2e-k8s-apiserver kubectl config use-context default --kubeconfig=/var/lib/kubernetes/admin.kubeconfig

    while ! cat <<EOF |  docker exec -i flannel-e2e-k8s-apiserver ${HYPERKUBE_CMD} kubectl --kubeconfig=/var/lib/kubernetes/admin.kubeconfig create -f - >/dev/null 2>/dev/null
apiVersion: v1
kind: Node
metadata:
  name: flannel1
  annotations:
    dummy: value
spec:
  podCIDR: 10.10.1.0/24
EOF
do
    sleep 1
done

cat <<EOF |  docker exec -i flannel-e2e-k8s-apiserver ${HYPERKUBE_CMD} kubectl --kubeconfig=/var/lib/kubernetes/admin.kubeconfig create -f - >/dev/null 2>/dev/null
apiVersion: v1
kind: Node
metadata:
  name: flannel2
  annotations:
    dummy: value
spec:
  podCIDR: 10.10.2.0/24
EOF
}

teardown_suite() {
    # Teardown the etcd server
    docker rm -f flannel-e2e-test-etcd >/dev/null
    docker rm -f flannel-e2e-k8s-apiserver >/dev/null
}

teardown() {
	docker rm -f flannel-e2e-test-flannel1 >/dev/null 2>/dev/null
	docker rm -f flannel-e2e-test-flannel2 >/dev/null 2>/dev/null
}

start_flannel() {
    local backend=$1

    flannel_conf="{ \"Network\": \"$FLANNEL_NET\", \"Backend\": { \"Type\": \"${backend}\" } }"
    dir=$(mktemp -d)

    docker exec -i flannel-e2e-k8s-apiserver cat /var/lib/kubernetes/admin.kubeconfig > $dir/admin.kubeconfig

    for host_num in 1 2; do
       docker rm -f flannel-e2e-test-flannel$host_num >/dev/null 2>/dev/null

       docker run -id --privileged \
	-v $dir:/var/lib/kubernetes/ \
        -e NODE_NAME=flannel$host_num \
        --name flannel-e2e-test-flannel$host_num \
        --entrypoint "/bin/sh" \
        $FLANNEL_DOCKER_IMAGE \
        -c "mkdir -p /etc/kube-flannel && \
            echo '$flannel_conf' > /etc/kube-flannel/net-conf.json && \
            /opt/bin/flanneld --kube-subnet-mgr --ip-masq --kubeconfig-file /var/lib/kubernetes/admin.kubeconfig --kube-api-url $k8s_endpt" >/dev/null

       while ! docker exec flannel-e2e-test-flannel$host_num ls /run/flannel/subnet.env >/dev/null 2>&1; do
         status=$(docker inspect --format='{{.State.Status}}' flannel-e2e-test-flannel$host_num)
         if [[ $status != "running" ]]; then
            docker logs flannel-e2e-test-flannel$host_num

            return
         fi

         sleep 0.1
       done
    done
}

create_ping_dest() {
    # add a dummy interface with $FLANNEL_SUBNET so we have a known working IP to ping
    for host_num in 1 2; do

       # Use declare to allow the host_num variable to be part of the ping_dest variable name. -g is needed to make it global
       declare -g ping_dest$host_num=$(docker "exec" --privileged flannel-e2e-test-flannel$host_num /bin/sh -c '\
		source /run/flannel/subnet.env && \
		ip link add name dummy0 type dummy && \
		ip addr add $FLANNEL_SUBNET dev dummy0 && ip link set dummy0 up && \
		echo $FLANNEL_SUBNET | cut -f 1 -d "/" ')
    done
}

test_vxlan() {
    start_flannel vxlan
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
    check_iptables
}

if [[ ${ARCH} == "amd64" ]]; then
test_udp() {
    start_flannel udp
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
    check_iptables
}
fi

test_host-gw() {
    start_flannel host-gw
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
    check_iptables
}

test_ipip() {
    start_flannel ipip
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
    check_iptables
}

test_public-ip-overwrite(){
  docker exec flannel-e2e-k8s-apiserver kubectl --kubeconfig=/var/lib/kubernetes/admin.kubeconfig annotate node flannel1 \
    flannel.alpha.coreos.com/public-ip-overwrite=172.18.0.2 >/dev/null 2>&1
  start_flannel vxlan
  assert_equals "172.18.0.2" \
    "$(docker exec flannel-e2e-k8s-apiserver kubectl --kubeconfig=/var/lib/kubernetes/admin.kubeconfig get node/flannel1 -o \
    jsonpath='{.metadata.annotations.flannel\.alpha\.coreos\.com/public-ip}' 2>/dev/null)" \
    "Overwriting public IP via annotation does not work"
  # Remove annotation to not break all other tests
  docker exec flannel-e2e-k8s-apiserver kubectl --kubeconfig=/var/lib/kubernetes/admin.kubeconfig annotate node flannel1 \
    flannel.alpha.coreos.com/public-ip-overwrite- >/dev/null 2>&1
}

test_wireguard() {
    start_flannel wireguard
    create_ping_dest # creates ping_dest1 and ping_dest2 variables
    pings
}

pings() {
    # ping in both directions
	assert "docker exec --privileged flannel-e2e-test-flannel1 /bin/ping -c 5 $ping_dest2" "Host 1 cannot ping host 2"
	assert "docker exec --privileged flannel-e2e-test-flannel2 /bin/ping -c 5 $ping_dest1" "Host 2 cannot ping host 1"
}

check_iptables() {
  read -r -d '' POSTROUTING_RULES_FLANNEL1 << EOM
-P POSTROUTING ACCEPT
-A POSTROUTING -m comment --comment "flanneld masq" -j FLANNEL-POSTRTG
-N FLANNEL-POSTRTG
-A FLANNEL-POSTRTG -m mark --mark 0x4000/0x4000 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s 10.10.0.0/16 -d 10.10.0.0/16 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s 10.10.0.0/16 ! -d 224.0.0.0/4 -m comment --comment "flanneld masq" -j MASQUERADE --random-fully
-A FLANNEL-POSTRTG ! -s 10.10.0.0/16 -d 10.10.1.0/24 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG ! -s 10.10.0.0/16 -d 10.10.0.0/16 -m comment --comment "flanneld masq" -j MASQUERADE --random-fully
EOM
  read -r -d '' POSTROUTING_RULES_FLANNEL2 << EOM
-P POSTROUTING ACCEPT
-A POSTROUTING -m comment --comment "flanneld masq" -j FLANNEL-POSTRTG
-N FLANNEL-POSTRTG
-A FLANNEL-POSTRTG -m mark --mark 0x4000/0x4000 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s 10.10.0.0/16 -d 10.10.0.0/16 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG -s 10.10.0.0/16 ! -d 224.0.0.0/4 -m comment --comment "flanneld masq" -j MASQUERADE --random-fully
-A FLANNEL-POSTRTG ! -s 10.10.0.0/16 -d 10.10.2.0/24 -m comment --comment "flanneld masq" -j RETURN
-A FLANNEL-POSTRTG ! -s 10.10.0.0/16 -d 10.10.0.0/16 -m comment --comment "flanneld masq" -j MASQUERADE --random-fully
EOM
  read -r -d '' FORWARD_RULES << EOM
-P FORWARD ACCEPT
-A FORWARD -m comment --comment "flanneld forward" -j FLANNEL-FWD
-N FLANNEL-FWD
-A FLANNEL-FWD -s 10.10.0.0/16 -m comment --comment "flanneld forward" -j ACCEPT
-A FLANNEL-FWD -d 10.10.0.0/16 -m comment --comment "flanneld forward" -j ACCEPT
EOM
  # check masquerade & forward rules
  assert_equals "$POSTROUTING_RULES_FLANNEL1" \
                "$(docker exec --privileged flannel-e2e-test-flannel1 /sbin/iptables -t nat -S POSTROUTING)
$(docker exec --privileged flannel-e2e-test-flannel1 /sbin/iptables -t nat -S FLANNEL-POSTRTG)" "Host 1 has not expected postrouting rules"
  assert_equals "$POSTROUTING_RULES_FLANNEL2" \
                "$(docker exec --privileged flannel-e2e-test-flannel2 /sbin/iptables -t nat -S POSTROUTING)
$(docker exec --privileged flannel-e2e-test-flannel2 /sbin/iptables -t nat -S FLANNEL-POSTRTG)" "Host 2 has not expected postrouting rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged flannel-e2e-test-flannel1 /sbin/iptables -t filter -S FORWARD)
$(docker exec --privileged flannel-e2e-test-flannel1 /sbin/iptables -t filter -S FLANNEL-FWD)" "Host 1 has not expected forward rules"
  assert_equals "$FORWARD_RULES" \
                "$(docker exec --privileged flannel-e2e-test-flannel2 /sbin/iptables -t filter -S FORWARD)
$(docker exec --privileged flannel-e2e-test-flannel2 /sbin/iptables -t filter -S FLANNEL-FWD)" "Host 2 has not expected forward rules"
}

test_manifest() {
    dir=$(mktemp -d)

    docker exec -i flannel-e2e-k8s-apiserver cat /var/lib/kubernetes/admin.kubeconfig > $dir/admin.kubeconfig
    # This just tests that the API server accepts the manifest, not that it actually acts on it correctly.
    assert "cat ../Documentation/kube-flannel.yml |  docker run -v $dir:/var/lib/kubernetes -i --rm --net=host ${HYPERKUBE_IMG}:v$K8S_VERSION-rancher1-linux-$ARCH ${HYPERKUBE_CMD} kubectl --kubeconfig=/var/lib/kubernetes/admin.kubeconfig create -f -"
}
