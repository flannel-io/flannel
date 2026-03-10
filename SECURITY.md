# Security Policy

## Supported Versions

The flannel project maintains security fixes for the **latest release** only.
Older releases are not actively patched. Users are encouraged to stay on the
latest stable release.

| Version        | Supported |
|----------------|-----------|
| Latest stable  | ✅ Yes    |
| Older versions | ❌ No     |

## Reporting a Vulnerability

The flannel maintainers take security vulnerabilities seriously and appreciate
responsible disclosure.

**Please do not report security vulnerabilities through public GitHub issues.**

To report a vulnerability, use **GitHub private vulnerability reporting**:
[https://github.com/flannel-io/flannel/security/advisories/new](https://github.com/flannel-io/flannel/security/advisories/new)

Please include as much detail as possible in your report:

- A description of the vulnerability and its potential impact
- Steps to reproduce the issue
- Any suggested mitigations or patches

## Disclosure Policy

We follow a **coordinated disclosure** process:

1. You report the vulnerability privately via GitHub's private vulnerability reporting.
2. The maintainers will acknowledge receipt of your report within **7 days**.
3. The maintainers will investigate and aim to produce a fix within **90 days**
   of the initial report, depending on severity and complexity.
4. A security advisory and patched release will be published simultaneously.
5. You are credited in the advisory (unless you prefer to remain anonymous).

If a vulnerability is not resolved within 90 days, we encourage reporters to
disclose publicly while coordinating with the maintainers to minimize user risk.

## Embargoed Vulnerability Announcements

To receive advance notifications of embargoed security vulnerabilities before
public disclosure, subscribe to the flannel distributors mailing list:

**[flannel-distributors-announce@googlegroups.com](https://groups.google.com/g/flannel-distributors-announce)**

This list is intended for distributors and downstream consumers of flannel who
need early access to security information to prepare patches or advisories.

## Security Advisories

Published security advisories for flannel can be found at:
[https://github.com/flannel-io/flannel/security/advisories](https://github.com/flannel-io/flannel/security/advisories)

## Scope

The following are considered in scope for vulnerability reports:

- The `flanneld` daemon and its backends (VXLAN, host-gw, WireGuard, etc.)
- The CNI plugin
- The flannel container image (e.g. secrets exposure, privilege escalation)
- The release workflow and supply chain (e.g. tampered artifacts)

The following are generally **out of scope**:

- Vulnerabilities in upstream dependencies (please report those upstream)
- Issues requiring physical access to the host
- Social engineering attacks

## Security-Related Configuration

Flannel runs as a privileged daemonset with access to the host network. Users
are advised to:

- Follow the [principle of least privilege](https://kubernetes.io/docs/concepts/security/rbac-good-practices/) when deploying flannel
- Keep flannel updated to the latest release
- Review the [flannel documentation](https://github.com/flannel-io/flannel/blob/master/Documentation/) for secure deployment guidance
