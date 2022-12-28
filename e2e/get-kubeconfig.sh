#!/usr/bin/env bash

set -e -o pipefail

export KUBECONFIG="${HOME}/.kube/config"

mkdir -vp "$(dirname $KUBECONFIG)"
while ! kubectl --insecure-skip-tls-verify get pods -A >/dev/null 2>&1 ; do
    echo 'Waiting for kubeconfig to become available...' >&2
    sleep 5
    docker exec local-leader kubectl config view --raw | sed -e "s/127.0.0.1/${KUBEHOST:=127.0.0.1}/g" > "${KUBECONFIG}"
done
