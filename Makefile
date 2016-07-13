.PHONY: all test cover gofmt gofmt-fix license-check

# Grab the absolute directory that contains this file.
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# These variables can be overridden by setting an environment variable.
PACKAGES?=pkg/ip subnet network remote
PACKAGES_EXPANDED=$(PACKAGES:%=github.com/coreos/flannel/%)

ifeq ($(ARCH),amd64)
	CC=gcc
endif
ifeq ($(ARCH),arm)
	CC=arm-linux-gnueabi-gcc
endif
ifeq ($(ARCH),arm64)
	CC=aarch64-linux-gnu-gcc
endif
ifeq ($(ARCH),ppc64le)
	CC=powerpc64le-linux-gnu-gcc
endif

REGISTRY?=gcr.io/google_containers
KUBE_CROSS_TAG=v1.6.2-2
GOARM=6
TEMP_DIR:=$(shell mktemp -d)
# Build the flannel image
#
# Usage:
# 	[TAG=0.5.5] [REGISTRY=gcr.io/google_containers] [ARCH=amd64] make build
TAG?=0.5.5
ARCH?=amd64

default: help
all: test				    ## Run all the tests
binary: flanneld  ## Create the flanneld binary

flanneld: $(shell find . -type f  -name '*.go')
	go build -o flanneld \
	  -ldflags "-extldflags -static -X github.com/coreos/flannel/version.Version=$(shell git describe --dirty)"

test:
	go test -cover $(PACKAGES_EXPANDED)

cover:
	#A single package must be given - e.g. 'PACKAGES=pkg/ip make cover'
	go test -coverprofile cover.out $(PACKAGES_EXPANDED)
	go tool cover -html=cover.out

# Throw an error if gofmt finds problems.
# "read" will return a failure return code if there is no output. This is inverted wth the "!"
gofmt:
	! gofmt -d $(PACKAGES) 2>&1 | read

gofmt-fix:
	gofmt -w $(PACKAGES)

license-check:
	dist/license-check.sh

# WHat does this create? Given an ARCH, from src to binary and container?
build:

#TODO artifacts dir, make it and
	docker run -it -v $(PWD):/flannel:ro gcr.io/google_containers/kube-cross:$(KUBE_CROSS_TAG) /bin/bash -c \
		&& cd /flannel && GOARM=$(GOARM) GOARCH=$(ARCH) CC=$(CC) CGO_ENABLED=1 make flanneld; file flanneld"


	curl http://www.netfilter.org/projects/iptables/files/iptables-1.4.21.tar.bz2 | tar -jxv
cd iptables-1.4.21
export CC=arm-linux-gnueabi-gcc
./configure \
    --prefix=/usr \
    --mandir=/usr/man \
    --disable-shared \
    --disable-devel \
    --disable-nftables \
    --enable-static \
    --host=amd64
make
cp iptables/xtables-multi

	# And build the image
	docker build -f Dockerfile.$(ARCH) -t $(REGISTRY)/flannel-$(ARCH):$(TAG) .


## Display this help text
help: # Some kind of magic from https://gist.github.com/rcmachado/af3db315e31383502660
	$(info Available targets)
	@awk '/^[a-zA-Z\-\_0-9]+:/ {								   \
		nb = sub( /^## /, "", helpMsg );							 \
		if(nb == 0) {												\
			helpMsg = $$0;											 \
			nb = sub( /^[^:]*:.* ## /, "", helpMsg );				  \
		}															\
		if (nb)													  \
			printf "\033[1;31m%-" width "s\033[0m %s\n", $$1, helpMsg; \
	}															  \
	{ helpMsg = $$0 }'											 \
	$(MAKEFILE_LIST)
