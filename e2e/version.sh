#!/bin/bash

set -e -o pipefail

export TAG=$(git describe --tags --always)
export ARCH=amd64
export FLANNEL_IMAGE_FILE=flanneld-${TAG}-${ARCH}
