# Release Process

This document describes how Flannel releases are created and published.

## Versioning

Flannel follows [Semantic Versioning](https://semver.org/):

- **Patch releases** (e.g., `v0.27.1 → v0.27.2`): Bug fixes and dependency updates. No breaking changes.
- **Minor releases** (e.g., `v0.26.x → v0.27.0`): New features, significant changes. May include minor breaking changes.

Flannel aims for approximately monthly releases, with patch releases issued as needed for important bug fixes or CVEs.

## Release Steps

1. **Prepare the release:** Ensure all intended pull requests are merged and the main branch CI is passing.

2. **Create a GitHub Release:**
   - Navigate to the [Releases page](https://github.com/flannel-io/flannel/releases) on GitHub.
   - Click "Draft a new release".
   - Create a new tag in the format `vX.Y.Z`.
   - Write release notes summarizing changes (new features, bug fixes, dependency updates, breaking changes if any).
   - Publish the release.

3. **Automated CI/CD:** Publishing a release triggers the [release GitHub Actions workflow](.github/workflows/release.yml), which automatically:
   - Builds multi-arch binaries for `amd64`, `arm`, `arm64`, `s390x`, `ppc64le`, and `riscv64`.
   - Builds and pushes multi-arch container images to [Docker Hub](https://hub.docker.com/r/flannel/flannel) and [GitHub Container Registry](https://ghcr.io/flannel-io/flannel).
   - Uploads binary artifacts (`flannel-*`) to the GitHub Release page.
   - Packages the Helm chart and `kube-flannel.yml` manifest and uploads them to the GitHub Release page.
   - Deploys the updated Helm chart to [GitHub Pages](https://flannel-io.github.io/flannel).
   - Generates build provenance attestations for container images.

## Release Artifacts

Each release publishes:

| Artifact | Description |
|----------|-------------|
| `flannel-<arch>` | Compiled `flanneld` binaries for each supported architecture |
| `kube-flannel.yml` | Kubernetes manifest for deploying Flannel |
| `flannel.tgz` | Helm chart package |
| Container images | Multi-arch images on Docker Hub and GHCR |

## Supported Architectures

`linux/amd64`, `linux/arm64`, `linux/arm`, `linux/s390x`, `linux/ppc64le`, `linux/riscv64`

## Security Releases

For releases addressing security vulnerabilities, the [SECURITY.md](../SECURITY.md) coordinated disclosure process is followed. Security releases are prioritized and may be issued outside the regular cadence.
