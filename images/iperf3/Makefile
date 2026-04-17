IPERF_IMG ?= iperf3:latest

ARCH ?= amd64

TEMP_DIR := $(shell mktemp -d)

ifeq ($(ARCH),amd64)
        BASEIMAGE=amd64/alpine:3.22.4
endif
ifeq ($(ARCH),arm64)
	BASEIMAGE=arm64v8/alpine:3.22.4
endif
ifeq ($(ARCH),ppc64le)
	BASEIMAGE=ppc64le/alpine:3.22.4
endif

.PHONY: all container

all: container

container:
	@echo "Copying files to $(TEMP_DIR)..."
	@cp ./* $(TEMP_DIR) || { echo "ERROR: Failed to copy files"; exit 1; }
	@echo "Preparing Dockerfile..."
	@cd $(TEMP_DIR) && sed -i 's|BASEIMAGE|$(BASEIMAGE)|g' Dockerfile || { echo "ERROR: Failed to prepare Dockerfile"; exit 1; }
	@echo "Building iperf3 container image..."
	@docker build --pull -t $(IPERF_IMG) -f $(TEMP_DIR)/Dockerfile $(TEMP_DIR) || { echo "ERROR: Docker build failed for iperf3"; exit 1; }
	@echo "Successfully built iperf3 image: $(IPERF_IMG)"
