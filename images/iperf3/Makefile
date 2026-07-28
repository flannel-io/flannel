IPERF_IMG ?= iperf3:latest

ARCH ?= amd64

ifeq ($(ARCH),amd64)
        BASEIMAGE=amd64/alpine:3.24@sha256:a0a65670a97d4d6ffc87eed1175c9c7d699813489e0913d96c7b917d7b51e456
endif
ifeq ($(ARCH),arm64)
	BASEIMAGE=arm64v8/alpine:3.24@sha256:65b6d5e8cc3010146b8b28cad29473b67883635fbedc9851f05ac83fb713ebdb
endif
ifeq ($(ARCH),ppc64le)
	BASEIMAGE=ppc64le/alpine:3.24@sha256:2b2dcda826915dfbd61de6b3b09d7dd6eb4252800b90c2ffa8ae9b040f6f4806
endif

.PHONY: all container

all: container

container:
	@echo "Building iperf3 container image..."
	@docker build --pull -t $(IPERF_IMG) --build-arg BASEIMAGE=$(BASEIMAGE) -f Dockerfile . || { echo "ERROR: Docker build failed for iperf3"; exit 1; }
	@echo "Successfully built iperf3 image: $(IPERF_IMG)"
