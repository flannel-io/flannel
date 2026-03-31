#!/usr/bin/env bash

set -e -o pipefail

source $(dirname $0)/version.sh

RELEASE="$(curl -sSL https://dl.k8s.io/release/stable.txt)"
ARCH="${ARCH:-amd64}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

curl -fsSLo "${TMP_DIR}/kubectl" "https://dl.k8s.io/release/${RELEASE}/bin/linux/${ARCH}/kubectl"
curl -fsSLo "${TMP_DIR}/kubectl.sha256" "https://dl.k8s.io/release/${RELEASE}/bin/linux/${ARCH}/kubectl.sha256"
echo "$(cat "${TMP_DIR}/kubectl.sha256")  ${TMP_DIR}/kubectl" | sha256sum --check --status

sudo install -m 0755 "${TMP_DIR}/kubectl" /usr/local/bin/kubectl
