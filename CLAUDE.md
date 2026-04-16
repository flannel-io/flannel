# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Flannel is a network fabric for Kubernetes that provides layer 3 IPv4 networking between nodes in a cluster. The main binary is `flanneld`, which runs on each host and allocates subnet leases from a larger preconfigured address space.

## Development Commands

### Building

Build for amd64 (in Docker container):
```bash
make dist/flanneld-amd64
```

Build for specific architecture:
```bash
ARCH=arm64 make dist/flanneld-arm64
ARCH=s390x make dist/flanneld-s390x
```

Build multi-arch image:
```bash
make buildx-create-builder  # One-time setup
make build-multi-arch
```

Build natively (requires CGO for amd64):
```bash
CGO_ENABLED=1 make dist/flanneld
```

### Testing

Run all tests (includes license check, gofmt, unit tests, e2e tests):
```bash
make test
```

Run unit tests only:
```bash
make unit-test
```

Run e2e tests:
```bash
make e2e-test
```

Run tests for specific packages:
```bash
TEST_PACKAGES="pkg/ip pkg/subnet" make unit-test
```

Check formatting:
```bash
make gofmt
```

Verify go modules:
```bash
make verify-modules
```

Update dependencies:
```bash
make deps  # runs go mod tidy && go mod vendor
```

### Local Development

Install binary locally (for quick iteration):
```bash
make install
```

## Architecture

### Core Components

**main.go**: Entry point that:
- Parses command-line flags (etcd endpoints, kube config, interface selection, etc.)
- Initializes the subnet manager (etcd or kube)
- Starts the backend network
- Manages traffic rules (iptables/nftables)

**Backend System** (`pkg/backend/`):
Backends are registered via init() and implement the network encapsulation layer:
- `vxlan`: VXLAN encapsulation (recommended, default port 8472)
- `host-gw`: Direct IP routes via layer 2
- `wireguard`: WireGuard encrypted tunnels
- `ipsec`: IPsec encrypted tunnels with strongSwan
- `udp`: Simple UDP encapsulation (debug only)
- `ipip`: IP-in-IP encapsulation
- `tencentvpc`: Tencent Cloud VPC integration
- `alloc`: Experimental allocation-only backend
- `extension`: External backend plugin support

Backends are imported with blank identifier in main.go to trigger registration:
```go
_ "github.com/flannel-io/flannel/pkg/backend/vxlan"
```

**Subnet Managers** (`pkg/subnet/`):
Two implementations for storing/retrieving network configuration:
- `etcd`: Uses etcd v3 as datastore (standalone deployments)
- `kube`: Uses Kubernetes API as datastore (kube subnet manager mode, no separate etcd needed)

**Traffic Management** (`pkg/trafficmngr/`):
Manages forwarding rules and masquerading using either iptables or nftables.

### Package Structure

- `pkg/ip`: IP address utilities and subnet operations
- `pkg/ipmatch`: IP address matching and selection (interface selection logic)
- `pkg/lease`: Subnet lease management
- `pkg/routing`: Route table management
- `pkg/ns`: Network namespace utilities
- `pkg/version`: Version information

### Build Architecture

**CGO_ENABLED**: Set to 1 for amd64 (enables UDP backend), 0 for other architectures.

**Cross-compilation**: Uses Docker with golang:1.25 image and qemu-user-static for cross-arch builds.

**Version embedding**: Git tag/commit is embedded via ldflags:
```
-ldflags '-X github.com/flannel-io/flannel/pkg/version.Version=$(TAG)'
```

### Testing Strategy

**Unit tests**: Run in Docker with NET_ADMIN and SYS_ADMIN capabilities to test network operations and namespace creation.

**E2E tests**: Use bash_unit framework with Docker Compose. Build test images with `dist/flanneld-e2e-$(TAG)-$(ARCH).docker` target.

**Functional tests**: Located in `dist/functional-test.sh` and `dist/functional-test-k8s.sh`.

## Key Configuration

**Go version**: 1.25 (see Makefile GO_VERSION)

**Supported architectures**: amd64, arm, arm64, s390x, ppc64le, riscv64

**Container registry**: quay.io/coreos/flannel (override with REGISTRY env var)

**Default test packages**: 
```
pkg/ip pkg/subnet pkg/subnet/etcd pkg/subnet/kube pkg/trafficmngr pkg/backend
```

## Release Process

See `Documentation/building.md` for full details. Key steps:

1. Create and push a git tag
2. Run `make release` to build all architectures
3. Run `make release-manifest` to generate kube-flannel.yml
4. Run `make release-helm` to package Helm chart
5. Upload artifacts from `dist/` to GitHub release

## Debugging

**Interface selection**: Flannel selects interfaces via `-iface`, `-iface-regex`, or `-iface-can-reach` flags. Logic in `pkg/ipmatch`.

**Backend issues**: Check backend-specific documentation in `Documentation/backends.md`.

**Subnet conflicts**: Network configuration stored in etcd or Kubernetes configmap/node annotations (depending on subnet manager).
