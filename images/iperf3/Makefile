IPERF_IMG ?= iperf3:latest

ARCH ?= amd64

ifeq ($(ARCH),amd64)
        BASEIMAGE=amd64/alpine:3.24@sha256:d5487879ebd5f0264ca2db5af02584e8d9090403f54cb3abcfcd7a5bcc1e7ebb
endif
ifeq ($(ARCH),arm64)
	BASEIMAGE=arm64v8/alpine:3.24@sha256:cc99de154757eb3d8b9200868d0bab96bff581a4f3000e62caed7baf46b41f95
endif
ifeq ($(ARCH),ppc64le)
	BASEIMAGE=ppc64le/alpine:3.24@sha256:a3ed54376ac872705532c49aef51a2ea0c92f1f824ec17a8f3c5f510dad619cd
endif

.PHONY: all container

all: container

container:
	@echo "Building iperf3 container image..."
	@docker build --pull -t $(IPERF_IMG) --build-arg BASEIMAGE=$(BASEIMAGE) -f Dockerfile . || { echo "ERROR: Docker build failed for iperf3"; exit 1; }
	@echo "Successfully built iperf3 image: $(IPERF_IMG)"
