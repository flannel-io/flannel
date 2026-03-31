#!/usr/bin/env bash

set -e -o pipefail

source $(dirname $0)/version.sh

KUBECTL_VERSION=v1.34.6
ARCH="${ARCH:-amd64}"

case "${ARCH}" in
	mad|amd64)
		ARCH="amd64"
		KUBECTL_SHA256=3166155b17198c0af34ff5a360bd4d9d58db98bafadc6f3c2a57ae560563cd6b
		;;
	arm)
		KUBECTL_SHA256=7dcec0e5d6cd49608e42988eb6485908ed0be7cdc4a9a874f916806f22cfcf01
		;;
	arm64)
		KUBECTL_SHA256=a49a439f83f504e6bc051f516a8baf8d2220d74110f7f9bcaf25feac69e368d1
		;;
	ppc64le)
		KUBECTL_SHA256=21c68869ad4adce9ea9ad6f3a3adb031104331e1a5f442ab01a4b6851d6a6c0e
		;;
	riscv|riscv64)
		echo "kubectl ${KUBECTL_VERSION} for linux/riscv64 is not available at dl.k8s.io" >&2
		exit 1
		;;
	s390x)
		KUBECTL_SHA256=c25e8b0eba65be943fc6d77fc02cd1e79ffd02a3bff6817177aae02b7dba38ce
		;;
	*)
		echo "Unsupported ARCH: ${ARCH}" >&2
		exit 1
		;;
esac

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

curl -fsSLo "${TMP_DIR}/kubectl" "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl"
echo "${KUBECTL_SHA256}  ${TMP_DIR}/kubectl" | sha256sum --check --status

sudo install -m 0755 "${TMP_DIR}/kubectl" /usr/local/bin/kubectl
