.PHONY: test e2e-test deps cover gofmt gofmt-fix license-check clean tar.gz docker-push release docker-push-all flannel-git docker-manifest-amend docker-manifest-push

# Registry used for publishing images
REGISTRY?=quay.io/coreos/flannel
QEMU_VERSION=v3.0.0

# Default tag and architecture. Can be overridden
TAG?=$(shell git describe --tags --dirty --always)
ARCH?=amd64
# Only enable CGO (and build the UDP backend) on AMD64
ifeq ($(ARCH),amd64)
	CGO_ENABLED=1
else
	CGO_ENABLED=0
endif

# Go version to use for builds
GO_VERSION=1.19

# K8s version used for Makefile helpers
K8S_VERSION=1.24.6

GOARM=7

# These variables can be overridden by setting an environment variable.
TEST_PACKAGES?=pkg/ip pkg/subnet pkg/subnet/etcd pkg/subnet/kube pkg/iptables pkg/backend
TEST_PACKAGES_EXPANDED=$(TEST_PACKAGES:%=github.com/flannel-io/flannel/%)
PACKAGES?=$(TEST_PACKAGES)
PACKAGES_EXPANDED=$(PACKAGES:%=github.com/flannel-io/flannel/%)

### BUILDING
clean:
	rm -f dist/flanneld*
	rm -f dist/*.aci
	rm -f dist/*.docker
	rm -f dist/*.tar.gz
	rm -f dist/qemu-*

dist/flanneld: $(shell find . -type f  -name '*.go')
	CGO_ENABLED=$(CGO_ENABLED) go build -o dist/flanneld \
	  -ldflags '-s -w -X github.com/flannel-io/flannel/version.Version=$(TAG) -extldflags "-static"'

dist/flanneld.exe: $(shell find . -type f  -name '*.go')
	CXX=x86_64-w64-mingw32-g++ CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows go build -o dist/flanneld.exe \
	  -ldflags '-s -w -X github.com/flannel-io/flannel/version.Version=$(TAG) -extldflags "-static"'

# This will build flannel natively using golang image
dist/flanneld-$(ARCH): deps dist/qemu-$(ARCH)-static
	# valid values for ARCH are [amd64 arm arm64 ppc64le s390x mips64le]
	docker run --rm -e CGO_ENABLED=$(CGO_ENABLED) -e GOARCH=$(ARCH) -e GOCACHE=/go \
		-u $(shell id -u):$(shell id -g) \
		-v $(CURDIR)/dist/qemu-$(ARCH)-static:/usr/bin/qemu-$(ARCH)-static \
		-v $(CURDIR):/go/src/github.com/flannel-io/flannel:ro \
		-v $(CURDIR)/dist:/go/src/github.com/flannel-io/flannel/dist \
		golang:$(GO_VERSION) /bin/bash -c '\
		cd /go/src/github.com/flannel-io/flannel && \
		make -e dist/flanneld && \
		mv dist/flanneld dist/flanneld-$(ARCH)'

## Create a docker image on disk for a specific arch and tag
image:	dist/flanneld-$(TAG)-$(ARCH).docker
dist/flanneld-$(TAG)-$(ARCH).docker: dist/flanneld-$(ARCH)
	docker build -f images/Dockerfile.$(ARCH) -t $(REGISTRY):$(TAG)-$(ARCH) .
	docker save -o dist/flanneld-$(TAG)-$(ARCH).docker $(REGISTRY):$(TAG)-$(ARCH)

# amd64 gets an image with the suffix too (i.e. it's the default)
ifeq ($(ARCH),amd64)
	docker build -f images/Dockerfile.$(ARCH) -t $(REGISTRY):$(TAG) .
endif

### TESTING
test: license-check gofmt deps verify-modules
	# Run the unit tests
	# NET_ADMIN capacity is required to do some network operation
	# SYS_ADMIN capacity is required to create network namespace
	docker run --cap-add=NET_ADMIN \
		--cap-add=SYS_ADMIN --rm \
		-v $(shell pwd):/go/src/github.com/flannel-io/flannel \
		golang:$(GO_VERSION) \
		/bin/bash -c 'cd /go/src/github.com/flannel-io/flannel && go test -v -cover -timeout 5m $(TEST_PACKAGES_EXPANDED)'

	# Test the docker-opts script
	cd dist; ./mk-docker-opts_tests.sh

	# Run the functional tests
	make e2e-test

e2e-test: bash_unit dist/flanneld-e2e-$(TAG)-$(ARCH).docker
	$(MAKE) -C images/iperf3 ARCH=$(ARCH)
	FLANNEL_DOCKER_IMAGE=$(REGISTRY):$(TAG)-$(ARCH) ./bash_unit dist/functional-test.sh
	FLANNEL_DOCKER_IMAGE=$(REGISTRY):$(TAG)-$(ARCH) ./bash_unit dist/functional-test-k8s.sh

k3s-e2e-test: bash_unit
	$(MAKE) -C images/iperf3 ARCH=$(ARCH)
	./bash_unit ./e2e/run-e2e-tests.sh

cover:
	# A single package must be given - e.g. 'PACKAGES=pkg/ip make cover'
	go test -coverprofile cover.out $(PACKAGES_EXPANDED)
	go tool cover -html=cover.out

license-check:
	# run license-check script
	dist/license-check.sh

# Throw an error if gofmt finds problems.
# "read" will return a failure return code if there is no output. This is inverted wth the "!"
gofmt:
	# Running gofmt... 
	docker run --rm -e CGO_ENABLED=$(CGO_ENABLED) -e GOARCH=$(ARCH) \
		-u $(shell id -u):$(shell id -g) \
		-v $(CURDIR):/go/src/github.com/flannel-io/flannel \
		-v $(CURDIR)/dist:/go/src/github.com/flannel-io/flannel/dist \
		golang:$(GO_VERSION) /bin/bash -c '\
		cd /go/src/github.com/flannel-io/flannel && \
		! gofmt -d $(PACKAGES) 2>&1 | read'

verify-modules:
	# Running verify-modules...
	docker run --rm -e CGO_ENABLED=$(CGO_ENABLED) -e GOARCH=$(ARCH) \
                -u $(shell id -u):$(shell id -g) \
                -v $(CURDIR):/go/src/github.com/flannel-io/flannel \
                -v $(CURDIR)/dist:/go/src/github.com/flannel-io/flannel/dist \
                golang:$(GO_VERSION) /bin/bash -c '\
                cd /go/src/github.com/flannel-io/flannel && \
		!go mod tidy 2>&1|read && \
		!go vet 2>&1|read'

gofmt-fix:
	docker run --rm -e CGO_ENABLED=$(CGO_ENABLED) -e GOARCH=$(ARCH) \
		-u $(shell id -u):$(shell id -g) \
		-v $(CURDIR):/go/src/github.com/flannel-io/flannel \
		-v $(CURDIR)/dist:/go/src/github.com/flannel-io/flannel/dist \
		golang:$(GO_VERSION) /bin/bash -c '\
		cd /go/src/github.com/flannel-io/flannel && \
		gofmt -w $(PACKAGES)'

bash_unit:
	wget https://raw.githubusercontent.com/pgrange/bash_unit/v2.0.1/bash_unit
	chmod +x bash_unit

# This will build flannel natively using golang image
dist/flanneld-e2e-$(TAG)-$(ARCH).docker:
ifneq ($(ARCH),amd64)
	$(MAKE) dist/qemu-$(ARCH)-static
endif
	# valid values for ARCH are [amd64 arm arm64 ppc64le s390x mips64le]
	docker run --rm -e GOARM=$(GOARM) -e CGO_ENABLED=$(CGO_ENABLED) -e GOCACHE=/go \
		-u $(shell id -u):$(shell id -g) \
		-v $(CURDIR):/go/src/github.com/flannel-io/flannel:ro \
		-v $(CURDIR)/dist:/go/src/github.com/flannel-io/flannel/dist \
		golang:$(GO_VERSION) /bin/bash -c '\
		cd /go/src/github.com/flannel-io/flannel && \
		make -e dist/flanneld && \
		mv dist/flanneld dist/flanneld-$(ARCH)'
	docker build -f images/Dockerfile.$(ARCH) -t $(REGISTRY):$(TAG)-$(ARCH) .

# Make a release after creating a tag
# To build cross platform Docker images, the qemu-static binaries are needed. On ubuntu "apt-get install  qemu-user-static"
release: tar.gz dist/qemu-s390x-static dist/qemu-ppc64le-static dist/qemu-arm64-static dist/qemu-arm-static dist/qemu-mips64le-static release-chart release-helm #release-tests
	ARCH=amd64 make dist/flanneld-$(TAG)-amd64.docker
	ARCH=arm make dist/flanneld-$(TAG)-arm.docker
	ARCH=arm64 make dist/flanneld-$(TAG)-arm64.docker
	ARCH=ppc64le make dist/flanneld-$(TAG)-ppc64le.docker
	ARCH=s390x make dist/flanneld-$(TAG)-s390x.docker
	ARCH=mips64le make dist/flanneld-$(TAG)-mips64le.docker
	@echo "Everything should be built for $(TAG)"
	@echo "Add all flanneld-* and *.tar.gz files from dist/ to the Github release"
	@echo "Use make docker-push-all to push the images to a registry"

release-chart:
	sed -i 's/^  newTag: .*/  newTag: $(TAG)/' Documentation/kustomization/kube-flannel/kustomization.yaml
	kubectl kustomize ./Documentation/kustomization/kube-flannel/ > dist/kube-flannel.yml
	sed -i 's/^  newTag: .*/  newTag: $(TAG)/' Documentation/kustomization/kube-flannel-psp/kustomization.yaml
	kubectl kustomize ./Documentation/kustomization/kube-flannel-psp/ > dist/kube-flannel-psp.yml

release-helm:
	sed -i '0,/^    tag: .*/s//    tag: $(TAG)/' ./chart/kube-flannel/values.yaml
	helm package ./chart/kube-flannel/ --destination dist/ --version $(TAG) --app-version $(TAG)
	mv dist/flannel-$(TAG).tgz dist/flannel.tgz

dist/qemu-%-static:
	if [ "$(@F)" = "qemu-amd64-static" ]; then \
		wget -O dist/qemu-amd64-static https://github.com/multiarch/qemu-user-static/releases/download/$(QEMU_VERSION)/qemu-x86_64-static; \
	elif [ "$(@F)" = "qemu-arm64-static" ]; then \
		wget -O dist/qemu-arm64-static https://github.com/multiarch/qemu-user-static/releases/download/$(QEMU_VERSION)/qemu-aarch64-static; \
	elif [ "$(@F)" = "qemu-mips64le-static" ]; then \
		wget -O dist/qemu-mips64le-static https://github.com/multiarch/qemu-user-static/releases/download/$(QEMU_VERSION)/qemu-mips64el-static; \
	else \
		wget -O dist/$(@F) https://github.com/multiarch/qemu-user-static/releases/download/$(QEMU_VERSION)/$(@F); \
	fi 

## Build a .tar.gz for the amd64 ppc64le arm arm64 mips64le flanneld binary
tar.gz:
	ARCH=amd64 make dist/flanneld-amd64
	tar --transform='flags=r;s|-amd64||' -zcvf dist/flannel-$(TAG)-linux-amd64.tar.gz -C dist flanneld-amd64 mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-linux-amd64.tar.gz
	ARCH=amd64 make dist/flanneld.exe
	tar --transform='flags=r;s|-amd64||' -zcvf dist/flannel-$(TAG)-windows-amd64.tar.gz -C dist flanneld.exe mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-windows-amd64.tar.gz
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
	ARCH=mips64le make dist/flanneld-mips64le
	tar --transform='flags=r;s|-mips64le||' -zcvf dist/flannel-$(TAG)-linux-mips64le.tar.gz -C dist flanneld-mips64le mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-linux-mips64le.tar.gz

release-tests: release-etcd-tests release-k8s-tests

release-etcd-tests: bash_unit
	# Run the functional tests with different etcd versions.
	ETCD_IMG="quay.io/coreos/etcd:latest"  ./bash_unit dist/functional-test.sh
	ETCD_IMG="quay.io/coreos/etcd:v3.2.7"  ./bash_unit dist/functional-test.sh
	# Etcd v2 docker image format is different. Override the etcd binary location so it works.
	ETCD_IMG="quay.io/coreos/etcd:v2.3.8"  ETCD_LOCATION=" " ./bash_unit dist/functional-test.sh

release-k8s-tests: bash_unit
	# Run the functional tests with different k8s versions. Currently these are the latest point releases.
	# This list should be updated during the release process.
	K8S_VERSION="1.25.2" HYPERKUBE_CMD=" " HYPERKUBE_APISERVER_CMD="kube-apiserver" ./bash_unit dist/functional-test-k8s.sh
	K8S_VERSION="1.24.6" HYPERKUBE_CMD=" " HYPERKUBE_APISERVER_CMD="kube-apiserver" ./bash_unit dist/functional-test-k8s.sh
	K8S_VERSION="1.23.12" HYPERKUBE_CMD=" " HYPERKUBE_APISERVER_CMD="kube-apiserver" ./bash_unit dist/functional-test-k8s.sh
	K8S_VERSION="1.22.15" HYPERKUBE_CMD=" " HYPERKUBE_APISERVER_CMD="kube-apiserver" ./bash_unit dist/functional-test-k8s.sh
	K8S_VERSION="1.17.3" HYPERKUBE_CMD=" " HYPERKUBE_APISERVER_CMD="kube-apiserver" ./bash_unit dist/functional-test-k8s.sh

docker-push: dist/flanneld-$(TAG)-$(ARCH).docker
	docker push $(REGISTRY):$(TAG)-$(ARCH)

docker-manifest-amend:
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create --amend $(REGISTRY):$(TAG) $(REGISTRY):$(TAG)-$(ARCH)

docker-manifest-push:
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest push --purge $(REGISTRY):$(TAG)

docker-push-all:
	ARCH=amd64 make docker-push docker-manifest-amend
	ARCH=arm make docker-push docker-manifest-amend
	ARCH=arm64 make docker-push docker-manifest-amend
	ARCH=ppc64le make docker-push docker-manifest-amend
	ARCH=s390x make docker-push docker-manifest-amend
	ARCH=mips64le make docker-push docker-manifest-amend
	make docker-manifest-push

flannel-git:
	ARCH=amd64 REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-amd64.docker docker-push docker-manifest-amend
	ARCH=arm REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-arm.docker docker-push docker-manifest-amend
	ARCH=arm64 REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-arm64.docker docker-push docker-manifest-amend
	ARCH=ppc64le REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-ppc64le.docker docker-push docker-manifest-amend
	ARCH=s390x REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-s390x.docker docker-push docker-manifest-amend
	ARCH=mips64le REGISTRY=quay.io/coreos/flannel-git make clean dist/flanneld-$(TAG)-mips64le.docker docker-push docker-manifest-amend
	REGISTRY=quay.io/coreos/flannel-git make docker-manifest-push

install:
	# This is intended as just a developer convenience to help speed up non-containerized builds
	# It is NOT how you install flannel
	CGO_ENABLED=$(CGO_ENABLED) go install -v github.com/flannel-io/flannel

minikube-start:
	minikube start --network-plugin cni

minikube-build-image:
	CGO_ENABLED=1 go build -v -o dist/flanneld-amd64
	# Make sure the minikube docker is being used "eval $(minikube docker-env)"
	sh -c 'eval $$(minikube docker-env) && docker build -f images/Dockerfile.amd64 -t flannel/minikube .'

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
	-e ETCD_UNSUPPORTED_ARCH=$(ARCH) \
	etcd \
	--advertise-client-urls "http://$(LOCAL_IP_ENV):2379,http://127.0.0.1:2379,http://$(LOCAL_IP_ENV):4001,http://127.0.0.1:4001" \
	--listen-client-urls "http://0.0.0.0:2379,http://0.0.0.0:4001"

stop-etcd:
	@-docker rm -f flannel-etcd

run-k8s-apiserver: stop-k8s-apiserver
	docker run --detach --net=host \
	  --name calico-k8s-apiserver \
	docker.io/rancher/hyperkube:v$(K8S_VERSION)-rancher1-linux-amd64 \
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

deps:
	go mod vendor
	go mod tidy
