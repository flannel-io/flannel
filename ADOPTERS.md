# Flannel Adopters

This document lists organizations known to use Flannel in production or as part of their infrastructure.

If your organization uses Flannel and is not listed here, please open a pull request to add it. This list helps the maintainers understand the project's reach and prioritize future work.

## Production Adopters

These organizations have strong, verifiable evidence of using Flannel in production: documented integrations, custom backends contributed to the codebase, or active maintainer involvement.

| Organization | Usage | More Information |
|---|---|---|
| SUSE / Rancher | Default CNI for [K3s](https://k3s.io) lightweight Kubernetes distribution | https://k3s.io |
| SUSE / Rancher | Default CNI for [RKE2](https://docs.rke2.io) Kubernetes distribution with Windows support | https://docs.rke2.io |
| Tigera | Networking layer in [Canal](https://projectcalico.docs.tigera.io/getting-started/kubernetes/flannel/flannel) (Flannel + Calico network policy) | https://projectcalico.docs.tigera.io |
| Tencent Cloud | Custom [Tencent VPC backend](pkg/backend/tencentvpc/) for Flannel running on Tencent Cloud infrastructure | https://cloud.tencent.com |
| Microsoft | Flannel support for Windows worker nodes in Kubernetes clusters (contributed Windows networking features) | https://www.microsoft.com |
| LinkedIn | Large-scale Kubernetes cluster networking (engineers contributed production fixes for large clusters) | https://www.linkedin.com |

## Organizations that Have Contributed to Flannel

The following organizations have employees who have contributed code, documentation, or other improvements to the Flannel project. They may also be using Flannel in production — please reach out and encourage them to confirm their usage above.

| Organization | Contribution Area |
|---|---|
| Airbnb | Kubernetes networking at scale |
| Alauda | Container platform networking |
| Alibaba Cloud | Cloud Kubernetes networking |
| ByteDance | Large-scale container orchestration networking |
| Caicloud | Container cloud platform |
| Canonical | Ubuntu-based Kubernetes deployments |
| Cerner (Oracle Health) | Healthcare Kubernetes infrastructure |
| Cloudbase Solutions | Windows container and Kubernetes networking |
| Collabora | Open source infrastructure |
| Fujitsu | Enterprise Kubernetes networking |
| Goodrain (Rainbond) | Container PaaS networking |
| Greenhouse.io | SaaS Kubernetes infrastructure |
| Hitachi Data Systems | Enterprise storage and networking |
| Hootsuite | Social media platform infrastructure |
| IBM | Enterprise Kubernetes cluster networking |
| LINE (LY Corporation) | Messaging platform Kubernetes networking |
| LMWN | Digital payment platform infrastructure |
| MicroFocus (OpenText) | Enterprise software Kubernetes |
| Mirakl | Marketplace platform infrastructure |
| NVIDIA | GPU-accelerated Kubernetes cluster networking |
| NTT | Cloud and telecommunications Kubernetes infrastructure |
| Pivotal (VMware) | Cloud-native application platform |
| Pure Software | ARM/embedded Kubernetes |
| Red Hat | Kubernetes networking contributions |
| Sematext | Observability platform Kubernetes |
| SofaScore | Sports data platform infrastructure |
| Spotify | Music streaming Kubernetes infrastructure |
| Stripe | Payment platform Kubernetes infrastructure |
| Transwarp | Big data platform networking |
| Tuenti (Telefónica) | Telecommunications Kubernetes |
| Vainu | Sales intelligence platform |
| VMware (Broadcom) | Kubernetes infrastructure and Tanzu platform networking |
| Zopa | Fintech platform infrastructure |

## How to Add Your Organization

To add your organization to this list:

1. Fork the [flannel repository](https://github.com/flannel-io/flannel)
2. Edit `ADOPTERS.md` to add your organization
3. Open a pull request

Please include:
- Organization name and link
- How you use Flannel (e.g., backend type, scale, cloud provider)
- Any relevant links (blog posts, case studies, etc.)
