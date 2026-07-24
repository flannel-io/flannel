#!/usr/bin/env bash

set -e -o pipefail

export KUBECONFIG="${HOME}/.kube/config"

mkdir -vp "$(dirname $KUBECONFIG)"
kind export kubeconfig --name e2e --kubeconfig "${KUBECONFIG}"
