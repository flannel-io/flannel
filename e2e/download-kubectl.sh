#!/usr/bin/env bash

set -e -o pipefail

source $(dirname $0)/version.sh

RELEASE="$(curl -sSL https://dl.k8s.io/release/stable.txt)"
pushd /usr/local/bin
sudo curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${RELEASE}/bin/linux/${ARCH:-amd64}/kubectl
sudo chmod +x kubectl
popd
