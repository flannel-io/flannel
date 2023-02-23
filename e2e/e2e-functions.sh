#!/usr/bin/env bash

# ---

e2e-wait-for-kubeconfig() {
    set -e -o pipefail
    # the `--insecure-skip-tls-verify` seems to be only needed here when run in dapper
    while ! kubectl --kubeconfig="${HOME}/.kube/config" --insecure-skip-tls-verify get pods -A >/dev/null 2>&1 ; do
        echo 'Waiting for kubeconfig to become available...' >&2
        sleep 5
        cluster-get-kubeconfig >/dev/null
    done
}
export -f e2e-wait-for-kubeconfig

e2e-count-ready-nodes() {
    kubectl --kubeconfig="${HOME}/.kube/config" get nodes -o json \
        | jq '.items[].status.conditions[] | select(.type == "Ready" and .status == "True") | .type' \
        | wc -l \
        | tr -d '[:space:]'
}
export -f e2e-count-ready-nodes

e2e-wait-for-nodes() {
    while [[ $(e2e-count-ready-nodes) -lt 2 ]]; do
        echo 'Waiting for nodes to be ready...' >&2
        echo "*** nodes:"
        kubectl --kubeconfig="${HOME}/.kube/config" get nodes
        # echo "*** events:"
        # kubectl --kubeconfig="${HOME}/.kube/config" get events --sort-by='.lastTimestamp' -A
        sleep 5
    done
    echo "*** nodes are ready:"
    kubectl --kubeconfig="${HOME}/.kube/config" get nodes
}
export -f e2e-wait-for-nodes

e2e-pod-ready() {
    kubectl --kubeconfig="${HOME}/.kube/config" get pods -A -o json \
        | jq ".items[].status.containerStatuses[] | select(.name == \"$1\") | .ready" 2>/dev/null
}
export -f e2e-pod-ready

e2e-wait-for-services() {
    for svc in ${WAIT_FOR_SERVICES:="coredns local-path-provisioner"}; do
        while [[ "$(e2e-pod-ready $svc)" != 'true' ]]; do
            echo "Waiting for service '$svc' to be ready..." >&2
            sleep 5
        done
        echo "Service '$svc' is ready"
    done
}
export -f e2e-wait-for-services

e2e-wait-for-test-pods() {
    for pod in ${WAIT_FOR_PODS:="multitool1 multitool2"}; do
        while [[ "$(e2e-pod-ready $pod)" != 'true' ]]; do
            echo "Waiting for pod '$pod' to be ready..." >&2
            sleep 5
        done
        echo "Pod '$pod' is ready"
    done
}
export -f e2e-wait-for-test-pods

e2e-wait-for-ping() {
    pod=$1
    ip=$2

    kubectl --kubeconfig="${HOME}/.kube/config" exec ${pod} -- ping -c 1 ${ip}
    result=$?
    while [ $result -ne 0 ]; do
        echo "Waiting for ${ip} to reply to ping from ${pod}..." >&2
        sleep 2
        kubectl --kubeconfig="${HOME}/.kube/config" exec ${pod} -- ping -c 1 ${ip}
        result=$?
    done
    echo "IP ${ip} is ready"
    return 0
}
export -f e2e-wait-for-ping

# ---

