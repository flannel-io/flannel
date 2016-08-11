.PHONY: test cover gofmt gofmt-fix license-check clean tar.gz docker-push release docker-push-all

# Registry used for publishing images
REGISTRY?=quay.io/coreos

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
GOARM=6
KUBE_CROSS_TAG=v1.6.2-2
IPTABLES_VERSION=1.4.21

dist/flanneld: $(shell find . -type f  -name '*.go')
	go build -o dist/flanneld \
	  -ldflags "-X github.com/coreos/flannel/version.Version=$(TAG)"

test: license-check gofmt
	go test -cover $(TEST_PACKAGES_EXPANDED)
	cd dist; ./mk-docker-opts_tests.sh

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
dist/flanneld-$(ARCH)-$(TAG).docker: dist/flanneld-$(ARCH) dist/iptables-$(ARCH)
	docker build -f Dockerfile.$(ARCH) -t $(REGISTRY)/flannel-$(ARCH):$(TAG) .
	docker save -o dist/flanneld-$(ARCH)-$(TAG).docker $(REGISTRY)/flannel-$(ARCH):$(TAG)

# amd64 gets an image with the suffix too (i.e. it's the default)
ifeq ($(ARCH),amd64)
	docker build -f Dockerfile.$(ARCH) -t $(REGISTRY)/flannel:$(TAG) .
endif

## Create an ACI on disk for a specific arch and tag
dist/flanneld-$(ARCH)-$(TAG).aci: dist/flanneld-$(ARCH)-$(TAG).docker
	docker2aci dist/flanneld-$(ARCH)-$(TAG).docker
	mv quay.io-coreos-flannel-$(ARCH)-$(TAG).aci dist/flanneld-$(ARCH)-$(TAG).aci
	actool patch-manifest --replace --capability=CAP_NET_ADMIN \
      --mounts=run-flannel,path=/run/flannel,readOnly=false:etc-ssl-etcd,path=/etc/ssl/etcd,readOnly=true:dev-net,path=/dev/net,readOnly=false \
      dist/flanneld-$(ARCH)-$(TAG).aci

docker-push: dist/flanneld-$(ARCH)-$(TAG).docker
	docker push $(REGISTRY)/flannel-$(ARCH):$(TAG)

# amd64 gets an image with the suffix too (i.e. it's the default)
ifeq ($(ARCH),amd64)
	docker push $(REGISTRY)/flannel:$(TAG)
endif

## Build an architecture specific flanneld binary
dist/flanneld-$(ARCH):
	# Build for other platforms with ARCH=$ARCH make build
	# valid values for $ARCH are [amd64 arm arm64 ppc64le]
	docker run -e CC=$(CC) -e GOARM=$(GOARM) -e GOARCH=$(ARCH) -it \
		-u $(shell id -u):$(shell id -u) \
	    -v ${PWD}:/go/src/github.com/coreos/flannel:ro \
        -v ${PWD}/dist:/go/src/github.com/coreos/flannel/dist \
	    gcr.io/google_containers/kube-cross:$(KUBE_CROSS_TAG) /bin/bash -c '\
		cd /go/src/github.com/coreos/flannel && \
		CGO_ENABLED=1 make -e dist/flanneld && \
		mv dist/flanneld dist/flanneld-$(ARCH) && \
		file dist/flanneld-$(ARCH)'

## Build an architecture specific iptables binary
dist/iptables-$(ARCH):
	docker run -e CC=$(CC) -e GOARM=$(GOARM) -e GOARCH=$(ARCH) -it \
			-u $(shell id -u):$(shell id -u) \
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

## Build a .tar.gz for the amd64 flanneld binary
tar.gz: dist/flannel-$(TAG)-linux-amd64.tar.gz
dist/flannel-$(TAG)-linux-amd64.tar.gz:
	ARCH=amd64 make dist/flanneld-amd64
	tar --transform='flags=r;s|-amd64||' -cvf dist/flannel-$(TAG)-linux-amd64.tar.gz -C dist flanneld-amd64 mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-linux-amd64.tar.gz

## Make a release after creating a tag
release: dist/flannel-$(TAG)-linux-amd64.tar.gz
	ARCH=amd64 make dist/flanneld-amd64-$(TAG).aci
	ARCH=arm make dist/flanneld-arm-$(TAG).aci
	ARCH=arm64 make dist/flanneld-arm64-$(TAG).aci
	ARCH=ppc64le make dist/flanneld-ppc64le-$(TAG).aci
	@echo "Everything should be built for $(TAG)"
	@echo "Add all *.aci, flanneld-* and *.tar.gz files from dist/ to the Github release"
	@echo "Use make docker-push-all to push the images to a registry"

docker-push-all:
	ARCH=amd64 make docker-push
	ARCH=arm make docker-push
	ARCH=arm64 make docker-push
	ARCH=ppc64le make docker-push
