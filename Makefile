.PHONY: test e2e-test cover gofmt gofmt-fix license-check clean tar.gz docker-push release docker-push-all flannel-git

# Registry used for publishing images
REGISTRY?=quay.io/coreos/flannel

# Default tag and architecture. Can be overridden
TAG?=$(shell git describe --tags --dirty)
ARCH?=amd64

# These variables can be overridden by setting an environment variable.
TEST_PACKAGES?=pkg/ip subnet remote
TEST_PACKAGES_EXPANDED=$(TEST_PACKAGES:%=github.com/coreos/flannel/%)
PACKAGES?=$(TEST_PACKAGES) network
PACKAGES_EXPANDED=$(PACKAGES:%=github.com/coreos/flannel/%)

# Set the (cross) compiler to use for different architectures
ifeq ($(ARCH),amd64)
	LIB_DIR=x86_64-linux-gnu
	CC=gcc
endif
ifeq ($(ARCH),arm)
	LIB_DIR=arm-linux-gnueabi
	CC=arm-linux-gnueabi-gcc
endif
ifeq ($(ARCH),arm64)
	LIB_DIR=aarch64-linux-gnu
	CC=aarch64-linux-gnu-gcc
endif
ifeq ($(ARCH),ppc64le)
	LIB_DIR=powerpc64le-linux-gnu
	CC=powerpc64le-linux-gnu-gcc
endif
GOARM=6
KUBE_CROSS_TAG=v1.6.2-2
IPTABLES_VERSION=1.4.21

dist/flanneld: $(shell find . -type f  -name '*.go')
	go build -o dist/flanneld \
	  -ldflags "-X github.com/coreos/flannel/version.Version=$(TAG)"

test: license-check gofmt
	go test -cover $(TEST_PACKAGES_EXPANDED)
	cd dist; ./mk-docker-opts_tests.sh

e2e-test: dist/flanneld-$(TAG)-$(ARCH).docker
	cd dist; ./functional-test.sh $(REGISTRY):$(TAG)-$(ARCH)

cover:
	# A single package must be given - e.g. 'PACKAGES=pkg/ip make cover'
	go test -coverprofile cover.out $(PACKAGES_EXPANDED)
	go tool cover -html=cover.out

# Throw an error if gofmt finds problems.
# "read" will return a failure return code if there is no output. This is inverted wth the "!"
gofmt:
	bash -c '! gofmt -d $(PACKAGES) 2>&1 | read'

gofmt-fix:
	gofmt -w $(PACKAGES)

license-check:
	./license-check.sh

clean:
	rm -f dist/flanneld*
	rm -f dist/iptables*
	rm -f dist/*.aci
	rm -f dist/*.docker
	rm -f dist/*.tar.gz

## Create a docker image on disk for a specific arch and tag
dist/flanneld-$(TAG)-$(ARCH).docker: dist/flanneld-$(ARCH) dist/iptables-$(ARCH) dist/libpthread.so.0-$(ARCH)
	docker build -f Dockerfile.$(ARCH) -t $(REGISTRY):$(TAG)-$(ARCH) .
	docker save -o dist/flanneld-$(TAG)-$(ARCH).docker $(REGISTRY):$(TAG)-$(ARCH)

# amd64 gets an image with the suffix too (i.e. it's the default)
ifeq ($(ARCH),amd64)
	docker build -f Dockerfile.$(ARCH) -t $(REGISTRY):$(TAG) .
endif

## Create an ACI on disk for a specific arch and tag
dist/flanneld-$(TAG)-$(ARCH).aci: dist/flanneld-$(TAG)-$(ARCH).docker
	docker2aci dist/flanneld-$(TAG)-$(ARCH).docker
	mv quay.io-coreos-flannel-$(TAG)-$(ARCH).aci dist/flanneld-$(TAG)-$(ARCH).aci
	actool patch-manifest --replace --capability=CAP_NET_ADMIN \
      --mounts=run-flannel,path=/run/flannel,readOnly=false:etc-ssl-etcd,path=/etc/ssl/etcd,readOnly=true:dev-net,path=/dev/net,readOnly=false \
      dist/flanneld-$(TAG)-$(ARCH).aci

docker-push: dist/flanneld-$(TAG)-$(ARCH).docker
	docker push $(REGISTRY):$(TAG)-$(ARCH)

# amd64 gets an image with the suffix too (i.e. it's the default)
ifeq ($(ARCH),amd64)
	docker push $(REGISTRY):$(TAG)
endif

## Build an architecture specific flanneld binary
dist/flanneld-$(ARCH):
	# Build for other platforms with ARCH=$$ARCH make build
	# valid values for $$ARCH are [amd64 arm arm64 ppc64le]
	docker run -e CC=$(CC) -e GOARM=$(GOARM) -e GOARCH=$(ARCH) \
		-u $(shell id -u):$(shell id -g) \
	    -v ${PWD}:/go/src/github.com/coreos/flannel:ro \
        -v ${PWD}/dist:/go/src/github.com/coreos/flannel/dist \
	    gcr.io/google_containers/kube-cross:$(KUBE_CROSS_TAG) /bin/bash -c '\
		cd /go/src/github.com/coreos/flannel && \
		CGO_ENABLED=1 make -e dist/flanneld && \
		mv dist/flanneld dist/flanneld-$(ARCH) && \
		file dist/flanneld-$(ARCH)'

## Busybox images are missing pthread. Pull it out of the kube-cross image
dist/libpthread.so.0-$(ARCH):
	docker run --rm -v `pwd`:/host gcr.io/google_containers/kube-cross:$(KUBE_CROSS_TAG) cp /lib/$(LIB_DIR)/libpthread.so.0 /host/dist/libpthread.so.0-$(ARCH)

## Build an architecture specific iptables binary
dist/iptables-$(ARCH):
	docker run -e CC=$(CC) -e GOARM=$(GOARM) -e GOARCH=$(ARCH) \
			-u $(shell id -u):$(shell id -g) \
            -v ${PWD}:/go/src/github.com/coreos/flannel:ro \
            -v ${PWD}/dist:/go/src/github.com/coreos/flannel/dist \
            gcr.io/google_containers/kube-cross:$(KUBE_CROSS_TAG) /bin/bash -c '\
            curl -sSL http://www.netfilter.org/projects/iptables/files/iptables-$(IPTABLES_VERSION).tar.bz2 | tar -jxv && \
            cd iptables-$(IPTABLES_VERSION) && \
            ./configure \
                --prefix=/usr \
                --mandir=/usr/man \
                --disable-shared \
                --disable-devel \
                --disable-nftables \
                --enable-static \
                --host=amd64 && \
            make && \
            cp iptables/xtables-multi /go/src/github.com/coreos/flannel/dist/iptables-$(ARCH) && \
            cd /go/src/github.com/coreos/flannel && \
            file dist/iptables-$(ARCH)'

## Build a .tar.gz for the amd64 ppc64le arm arm64 flanneld binary
tar.gz:	
	ARCH=amd64 make dist/flanneld-amd64
	tar --transform='flags=r;s|-amd64||' -zcvf dist/flannel-$(TAG)-linux-amd64.tar.gz -C dist flanneld-amd64 mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-linux-amd64.tar.gz
	ARCH=ppc64le make dist/flanneld-ppc64le
	tar --transform='flags=r;s|-ppc64le||' -zcvf dist/flannel-$(TAG)-linux-ppc64le.tar.gz -C dist flanneld-ppc64le mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-linux-ppc64le.tar.gz
	ARCH=arm make dist/flanneld-arm
	tar --transform='flags=r;s|-arm||' -zcvf dist/flannel-$(TAG)-linux-arm.tar.gz -C dist flanneld-arm mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-linux-arm.tar.gz
	ARCH=arm64 make dist/flanneld-arm64
	tar --transform='flags=r;s|-arm64||' -zcvf dist/flannel-$(TAG)-linux-arm64.tar.gz -C dist flanneld-arm64 mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-linux-arm64.tar.gz

## Make a release after creating a tag
release: dist/flannel-$(TAG)-linux-amd64.tar.gz
	ARCH=amd64 make dist/flanneld-$(TAG)-amd64.aci
	ARCH=arm make dist/flanneld-$(TAG)-arm.aci
	ARCH=arm64 make dist/flanneld-$(TAG)-arm64.aci
	ARCH=ppc64le make dist/flanneld-$(TAG)-ppc64le.aci
	@echo "Everything should be built for $(TAG)"
	@echo "Add all *.aci, flanneld-* and *.tar.gz files from dist/ to the Github release"
	@echo "Use make docker-push-all to push the images to a registry"

docker-push-all:
	ARCH=amd64 make docker-push
	ARCH=arm make docker-push
	ARCH=arm64 make docker-push
	ARCH=ppc64le make docker-push

flannel-git:
	ARCH=amd64 REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-amd64.docker docker-push
	docker build -f Dockerfile.amd64 -t quay.io/coreos/flannel-git .
	docker push quay.io/coreos/flannel-git
	ARCH=arm REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-arm.docker docker-push
	ARCH=arm64 REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-arm64.docker docker-push
	ARCH=ppc64le REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-ppc64le.docker docker-push
