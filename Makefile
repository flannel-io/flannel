.PHONY: test unit-test e2e-test deps cover gofmt gofmt-fix license-check clean tar.gz release buildx-create-builder build-multi-arch

# Registry used for publishing images
REGISTRY?=quay.io/coreos/flannel
QEMU_VERSION=v3.0.0
BASH_UNIT_VERSION=v2.3.0

# Default tag and architecture. Can be overridden
TAG?=$(shell git describe --tags --always)
ARCH?=amd64
# Only enable CGO (and build the UDP backend) on AMD64
ifeq ($(ARCH),amd64)
	CGO_ENABLED=1
else
	CGO_ENABLED=0
endif

# Go version to use for builds
GO_VERSION=1.23

# K8s version used for Makefile helpers
K8S_VERSION=1.29.8

GOARM=7

# These variables can be overridden by setting an environment variable.
TEST_PACKAGES?=pkg/ip pkg/subnet pkg/subnet/etcd pkg/subnet/kube pkg/trafficmngr pkg/backend
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
	  -ldflags '-s -w -X github.com/flannel-io/flannel/pkg/version.Version=$(TAG) -extldflags "-static"'

dist/flanneld.exe: $(shell find . -type f  -name '*.go')
	CXX=x86_64-w64-mingw32-g++ CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows go build -o dist/flanneld.exe \
	  -ldflags '-s -w -X github.com/flannel-io/flannel/pkg/version.Version=$(TAG) -extldflags "-static"'

# This will build flannel natively using golang image
dist/flanneld-$(ARCH): deps dist/qemu-$(ARCH)-static
	# valid values for ARCH are [amd64 arm arm64 ppc64le s390x riscv64]
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
image:	deps dist/flanneld-$(TAG)-$(ARCH).docker
dist/flanneld-$(TAG)-$(ARCH).docker:
	docker buildx build -f images/Dockerfile --platform=$(ARCH) --build-arg TAG=$(TAG) -t $(REGISTRY):$(TAG)-$(ARCH) --load .
	docker save -o dist/flanneld-$(TAG)-$(ARCH).docker $(REGISTRY):$(TAG)-$(ARCH)

# amd64 gets an image without the suffix too (i.e. it's the default)
ifeq ($(ARCH),amd64)
	docker build -f images/Dockerfile --platform=$(ARCH) --build-arg TAG=$(TAG) -t $(REGISTRY):$(TAG) .
endif

### TESTING
test: license-check gofmt deps verify-modules
	make unit-test

	# Test the docker-opts script
	cd dist; ./mk-docker-opts_tests.sh

	# Run the functional tests
	make e2e-test

unit-test: 
	# Run the unit tests
	# NET_ADMIN capacity is required to do some network operation
	# SYS_ADMIN capacity is required to create network namespace
	docker run --cap-add=NET_ADMIN \
		--cap-add=SYS_ADMIN --rm \
		-v $(shell pwd):/go/src/github.com/flannel-io/flannel \
		golang:$(GO_VERSION) \
		/bin/bash -c 'cd /go/src/github.com/flannel-io/flannel && go test -v -cover -timeout 5m $(TEST_PACKAGES_EXPANDED)'

e2e-test: bash_unit dist/flanneld-e2e-$(TAG)-$(ARCH).docker
	$(MAKE) -C images/iperf3 ARCH=$(ARCH)
	FLANNEL_DOCKER_IMAGE=$(REGISTRY):$(TAG)-$(ARCH) ./bash_unit dist/functional-test.sh
	FLANNEL_DOCKER_IMAGE=$(REGISTRY):$(TAG)-$(ARCH) ./bash_unit dist/functional-test-k8s.sh

k3s-e2e-test: bash_unit dist/flanneld-e2e-$(TAG)-$(ARCH).docker
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


bash_unit:
	wget https://raw.githubusercontent.com/pgrange/bash_unit/$(BASH_UNIT_VERSION)/bash_unit
	chmod +x bash_unit

# This will build flannel natively using golang image
dist/flanneld-e2e-$(TAG)-$(ARCH).docker:
ifneq ($(ARCH),amd64)
	$(MAKE) dist/qemu-$(ARCH)-static
endif
	# valid values for ARCH are [amd64 arm arm64 ppc64le s390x riscv64]
	docker run --rm -e GOARM=$(GOARM) -e CGO_ENABLED=$(CGO_ENABLED) -e GOCACHE=/go \
		-u $(shell id -u):$(shell id -g) \
		-v $(CURDIR):/go/src/github.com/flannel-io/flannel:ro \
		-v $(CURDIR)/dist:/go/src/github.com/flannel-io/flannel/dist \
		golang:$(GO_VERSION) /bin/bash -c '\
		cd /go/src/github.com/flannel-io/flannel && \
		make -e dist/flanneld && \
		mv dist/flanneld dist/flanneld-$(ARCH)'
	docker build -f images/Dockerfile --platform=$(ARCH) --build-arg TAG=$(TAG) -t $(REGISTRY):$(TAG)-$(ARCH) .

# Make a release after creating a tag
# To build cross platform Docker images, the qemu-static binaries are needed. On ubuntu "apt-get install  qemu-user-static"
release: tar.gz dist/qemu-s390x-static dist/qemu-ppc64le-static dist/qemu-arm64-static dist/qemu-arm-static dist/qemu-riscv64-static release-chart release-helm
	ARCH=amd64 make dist/flanneld-$(TAG)-amd64.docker
	ARCH=arm make dist/flanneld-$(TAG)-arm.docker
	ARCH=arm64 make dist/flanneld-$(TAG)-arm64.docker
	ARCH=ppc64le make dist/flanneld-$(TAG)-ppc64le.docker
	ARCH=s390x make dist/flanneld-$(TAG)-s390x.docker
	ARCH=riscv64 make dist/flanneld-$(TAG)-riscv64.docker
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
	helm package ./chart/kube-flannel/ --destination chart/ --version $(TAG) --app-version $(TAG)
	cp chart/flannel-$(TAG).tgz dist/flannel.tgz
	mv chart/flannel-$(TAG).tgz chart/flannel.tgz
	wget https://flannel-io.github.io/flannel/index.yaml -O chart/index.yaml
	helm repo index --merge chart/index.yaml --url https://github.com/flannel-io/flannel/releases/download/$(TAG)/ chart/

dist/qemu-%-static:
	if [ "$(@F)" = "qemu-amd64-static" ]; then \
		wget -O dist/qemu-amd64-static https://github.com/multiarch/qemu-user-static/releases/download/$(QEMU_VERSION)/qemu-x86_64-static; \
	elif [ "$(@F)" = "qemu-arm64-static" ]; then \
		wget -O dist/qemu-arm64-static https://github.com/multiarch/qemu-user-static/releases/download/$(QEMU_VERSION)/qemu-aarch64-static; \
	else \
		wget -O dist/$(@F) https://github.com/multiarch/qemu-user-static/releases/download/$(QEMU_VERSION)/$(@F); \
	fi 

## Build a .tar.gz for the amd64 ppc64le arm arm64 riscv64 flanneld binary
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
	ARCH=riscv64 make dist/flanneld-riscv64
	tar --transform='flags=r;s|-riscv64||' -zcvf dist/flannel-$(TAG)-linux-riscv64.tar.gz -C dist flanneld-riscv64 mk-docker-opts.sh ../README.md
	tar -tvf dist/flannel-$(TAG)-linux-riscv64.tar.gz

install:
	# This is intended as just a developer convenience to help speed up non-containerized builds
	# It is NOT how you install flannel
	CGO_ENABLED=$(CGO_ENABLED) go install -v github.com/flannel-io/flannel

deps:
	go mod tidy
	go mod vendor

buildx-create-builder:
	docker buildx create --name mybuilder --use --bootstrap

build-multi-arch:
	docker buildx build  --platform linux/amd64,linux/arm64,linux/arm,linux/s390x,linux/ppc64le,linux/riscv64 -t $(REGISTRY):$(TAG) -f images/Dockerfile --build-arg TAG=$(TAG) -o type=oci,dest=dist/flannel_oci.tar --progress plain .

