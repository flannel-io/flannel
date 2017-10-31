.PHONY: test e2e-test cover gofmt gofmt-fix license-check clean tar.gz docker-push release docker-push-all flannel-git

# Registry used for publishing images
REGISTRY?=quay.io/coreos/flannel

# Default tag and architecture. Can be overridden
TAG?=$(shell git describe --tags --dirty)
ARCH?=amd64

# These variables can be overridden by setting an environment variable.
TEST_PACKAGES?=pkg/ip subnet subnet/etcdv2 network backend/hostgw
TEST_PACKAGES_EXPANDED=$(TEST_PACKAGES:%=github.com/coreos/flannel/%)
PACKAGES?=$(TEST_PACKAGES) network
PACKAGES_EXPANDED=$(PACKAGES:%=github.com/coreos/flannel/%)

# Set the (cross) compiler to use for different architectures
ifeq ($(ARCH),amd64)
	CC=gcc
endif
ifeq ($(ARCH),arm)
	CC=arm-linux-gnueabihf-gcc
endif
ifeq ($(ARCH),arm64)
	CC=aarch64-linux-gnu-gcc
endif
ifeq ($(ARCH),ppc64le)
	CC=powerpc64le-linux-gnu-gcc
endif
ifeq ($(ARCH),s390x)
	CC=s390x-linux-gnu-gcc
endif

GOARM=7

# List images with gcloud alpha container images list-tags gcr.io/google_containers/kube-cross
KUBE_CROSS_TAG=v1.8.3-1
IPTABLES_VERSION=1.4.21

dist/flanneld: $(shell find . -type f  -name '*.go')
	go build -o dist/flanneld \
	  -ldflags "-s -w -X github.com/coreos/flannel/version.Version=$(TAG)"

test: license-check gofmt
	# Run the unit tests
	docker run --cap-add=NET_ADMIN --rm -v $(shell pwd):/go/src/github.com/coreos/flannel golang:1.8.3 go test -v -cover $(TEST_PACKAGES_EXPANDED)

	# Test the docker-opts script
	cd dist; ./mk-docker-opts_tests.sh

	# Run the functional tests
	make e2e-test

e2e-test: bash_unit dist/flanneld-e2e-$(TAG)-$(ARCH).docker
	$(MAKE) -C images/iperf3 ARCH=$(ARCH)
	FLANNEL_DOCKER_IMAGE=$(REGISTRY):$(TAG)-$(ARCH) ./bash_unit dist/functional-test.sh
	FLANNEL_DOCKER_IMAGE=$(REGISTRY):$(TAG)-$(ARCH) ./bash_unit dist/functional-test-k8s.sh

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

update-glide:
	# go get -d -u github.com/Masterminds/glide
	glide update --strip-vendor
	# go get -d -u github.com/sgotti/glide-vc
	glide vc --only-code --no-tests

bash_unit:
	wget https://raw.githubusercontent.com/pgrange/bash_unit/v1.6.0/bash_unit
	chmod +x bash_unit

clean:
	rm -f dist/flanneld*
	rm -f dist/*.docker
	rm -f dist/*.tar.gz

## Create a docker image on disk for a specific arch and tag
dist/flanneld-$(TAG)-$(ARCH).docker: dist/flanneld-$(ARCH) dist/iptables-$(ARCH)
	docker build -f Dockerfile.$(ARCH) -t $(REGISTRY):$(TAG)-$(ARCH) .
	docker save -o dist/flanneld-$(TAG)-$(ARCH).docker $(REGISTRY):$(TAG)-$(ARCH)

# amd64 gets an image with the suffix too (i.e. it's the default)
ifeq ($(ARCH),amd64)
	docker build -f Dockerfile.$(ARCH) -t $(REGISTRY):$(TAG) .
endif

# This will build flannel natively using golang image
dist/flanneld-e2e-$(TAG)-$(ARCH).docker:
	# valid values for ARCH are [amd64 arm arm64 ppc64le s390x]
	docker run -e GOARM=$(GOARM) \
		-u $(shell id -u):$(shell id -g) \
		-v $(CURDIR):/go/src/github.com/coreos/flannel:ro \
		-v $(CURDIR)/dist:/go/src/github.com/coreos/flannel/dist \
		golang:1.8.3 /bin/bash -c '\
		cd /go/src/github.com/coreos/flannel && \
		CGO_ENABLED=1 make -e dist/flanneld && \
		mv dist/flanneld dist/flanneld-$(ARCH)'
	docker build -f Dockerfile.$(ARCH) -t $(REGISTRY):$(TAG)-$(ARCH) .

docker-push: dist/flanneld-$(TAG)-$(ARCH).docker
	docker push $(REGISTRY):$(TAG)-$(ARCH)

# amd64 gets an image with the suffix too (i.e. it's the default)
ifeq ($(ARCH),amd64)
	docker push $(REGISTRY):$(TAG)
endif

## Build an architecture specific flanneld binary
dist/flanneld-$(ARCH):
	# Build for other platforms with 'ARCH=$$ARCH make dist/flanneld-$$ARCH'
	# valid values for $$ARCH are [amd64 arm arm64 ppc64le s390x]
	docker run -e CC=$(CC) -e GOARM=$(GOARM) -e GOARCH=$(ARCH) \
		-u $(shell id -u):$(shell id -g) \
	    -v $(CURDIR):/go/src/github.com/coreos/flannel:ro \
        -v $(CURDIR)/dist:/go/src/github.com/coreos/flannel/dist \
	    gcr.io/google_containers/kube-cross:$(KUBE_CROSS_TAG) /bin/bash -c '\
		cd /go/src/github.com/coreos/flannel && \
		CGO_ENABLED=1 make -e dist/flanneld && \
		mv dist/flanneld dist/flanneld-$(ARCH) && \
		file dist/flanneld-$(ARCH)'

## Build an architecture specific iptables binary
dist/iptables-$(ARCH):
	docker run -e CC=$(CC) -e GOARM=$(GOARM) -e GOARCH=$(ARCH) \
			-u $(shell id -u):$(shell id -g) \
            -v $(CURDIR):/go/src/github.com/coreos/flannel:ro \
            -v $(CURDIR)/dist:/go/src/github.com/coreos/flannel/dist \
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
	ARCH=s390x make dist/flanneld-s390x
	tar --transform='flags=r;s|-s390x||' -zcvf dist/flannel-$(TAG)-linux-s390x.tar.gz -C dist flanneld-s390x mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-linux-s390x.tar.gz

## Make a release after creating a tag
release: tar.gz release-tests
	ARCH=amd64 make dist/flanneld-$(TAG)-amd64.docker
	ARCH=arm make dist/flanneld-$(TAG)-arm.docker
	ARCH=arm64 make dist/flanneld-$(TAG)-arm64.docker
	ARCH=ppc64le make dist/flanneld-$(TAG)-ppc64le.docker
	ARCH=s390x make dist/flanneld-$(TAG)-s390x.docker
	@echo "Everything should be built for $(TAG)"
	@echo "Add all flanneld-* and *.tar.gz files from dist/ to the Github release"
	@echo "Use make docker-push-all to push the images to a registry"

release-tests: bash_unit
	# Run the functional tests with different etcd versions.
	ETCD_IMG="quay.io/coreos/etcd:latest"  ./bash_unit dist/functional-test.sh
	ETCD_IMG="quay.io/coreos/etcd:v3.2.7"  ./bash_unit dist/functional-test.sh
	ETCD_IMG="quay.io/coreos/etcd:v3.1.10" ./bash_unit dist/functional-test.sh
	ETCD_IMG="quay.io/coreos/etcd:v3.0.17" ./bash_unit dist/functional-test.sh
	# Etcd v2 docker image format is different. Override the etcd binary location so it works.
	ETCD_IMG="quay.io/coreos/etcd:v2.3.8"  ETCD_LOCATION=" " ./bash_unit dist/functional-test.sh

	# Run the functional tests with different k8s versions. Currently these are the latest point releases.
	# This list should be updated during the release process.
	K8S_VERSION="1.7.6"  ./bash_unit dist/functional-test-k8s.sh
	K8S_VERSION="1.6.10" ./bash_unit dist/functional-test-k8s.sh
	# K8S_VERSION="1.5.7"  ./bash_unit dist/functional-test-k8s.sh   #kube-flannel.yml is incompatible
	# K8S_VERSION="1.4.12" ./bash_unit dist/functional-test-k8s.sh   #kube-flannel.yml is incompatible
	# K8S_VERSION="1.3.10" ./bash_unit dist/functional-test-k8s.sh   #kube-flannel.yml is incompatible

docker-push-all:
	ARCH=amd64 make docker-push
	ARCH=arm make docker-push
	ARCH=arm64 make docker-push
	ARCH=ppc64le make docker-push
	ARCH=s390x make docker-push

flannel-git:
	ARCH=amd64 REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-amd64.docker docker-push
	docker build -f Dockerfile.amd64 -t quay.io/coreos/flannel-git .
	docker push quay.io/coreos/flannel-git
	ARCH=arm REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-arm.docker docker-push
	ARCH=arm64 REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-arm64.docker docker-push
	ARCH=ppc64le REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-ppc64le.docker docker-push
	ARCH=s390x REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-s390x.docker docker-push

install:
	# This is intended as just a developer convenience to help speed up non-containerized builds
	# It is NOT how you install flannel
	CGO_ENABLED=1 go install -v github.com/coreos/flannel

minikube-start:
	minikube start --network-plugin cni

minikube-build-image: dist/iptables-amd64
	CGO_ENABLED=1 go build -v -o dist/flanneld-amd64
	# Make sure the minikube docker is being used "eval $(minikube docker-env)"
	sh -c 'eval $$(minikube docker-env) && docker build -f Dockerfile.amd64 -t flannel/minikube .'

minikube-deploy-flannel:
	kubectl apply -f Documentation/minikube.yml

minikube-remove-flannel:
	kubectl delete -f Documentation/minikube.yml

minikube-restart-pod:
	# Use this to pick up a new image
	kubectl delete pods -l app=flannel --grace-period=0

kubernetes-logs:
	kubectl logs `kubectl get po -l app=flannel -o=custom-columns=NAME:metadata.name --no-headers=true` -c kube-flannel -f

LOCAL_IP_ENV?=$(shell ip route get 8.8.8.8 | head -1 | awk '{print $$7}')
run-etcd: stop-etcd
	docker run --detach \
	-p 2379:2379 \
	--name flannel-etcd quay.io/coreos/etcd \
	etcd \
	--advertise-client-urls "http://$(LOCAL_IP_ENV):2379,http://127.0.0.1:2379,http://$(LOCAL_IP_ENV):4001,http://127.0.0.1:4001" \
	--listen-client-urls "http://0.0.0.0:2379,http://0.0.0.0:4001"

stop-etcd:
	@-docker rm -f flannel-etcd

K8S_VERSION=v1.6.6
run-k8s-apiserver: stop-k8s-apiserver
	docker run --detach --net=host \
	  --name calico-k8s-apiserver \
  	gcr.io/google_containers/hyperkube-amd64:$(K8S_VERSION) \
		  /hyperkube apiserver --etcd-servers=http://$(LOCAL_IP_ENV):2379 \
		  --service-cluster-ip-range=10.101.0.0/16

stop-k8s-apiserver:
	@-docker rm -f calico-k8s-apiserver

run-local-kube-flannel-with-prereqs: run-etcd run-k8s-apiserver dist/flanneld
	while ! kubectl apply -f dist/fake-node.yaml; do sleep 1; done
	$(MAKE) run-local-kube-flannel

run-local-kube-flannel:
	# Currently this requires the netconf to be in /etc/kube-flannel/net-conf.json
	sudo NODE_NAME=test dist/flanneld --kube-subnet-mgr --kube-api-url http://127.0.0.1:8080