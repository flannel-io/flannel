# Install the all-in-one binary so we can copy our run-time images into the image
# which helps avoid pulling them when running e2e tests.
ARG SLES="registry.suse.com/suse/sle15:15.7"
FROM ${SLES} AS k3s
ARG ARCH
ARG K3S_VERSION="v1.34.6+k3s1"
ARG CNI_VERSION="v1.9.1"
RUN set -e -x \
 && zypper -n in \
    ca-certificates \
    curl \
    tar gzip\
    iptables \
    nftables \
    iproute2 \
    iputils \
 && if [ "${ARCH?required}" != "amd64" ]; then \
        K3S_SUFFIX="-${ARCH}"; \
    fi \
 && K3S_BIN="k3s${K3S_SUFFIX}" \
 && case "${ARCH}" in \
      amd64) K3S_SHA256="7c81556ac1212213a07180f92eca57db8caef886e58bbad15e9dada10782c10d" ;; \
      arm)   K3S_SHA256="5a53717e45db133408cecc7dfe4f42f076e71333df0ca0a44b79ff1568a5ee36" ;; \
      arm64) K3S_SHA256="2695afc60eea47880ba71c9a01412693b2246c1c76623a352a727db566a6d8b5" ;; \
      *) echo "unsupported ARCH: ${ARCH}" >&2; exit 1 ;; \
    esac \
 && curl -fsSL "https://github.com/rancher/k3s/releases/download/${K3S_VERSION}/${K3S_BIN}" -o /bin/k3s \
 && { printf '%s  %s\n' "${K3S_SHA256}" "/bin/k3s" | sha256sum -c - || { echo "ERROR: K3S checksum verification failed for ${ARCH}. Expected: ${K3S_SHA256}"; exit 1; }; } \
 && chmod +x /bin/k3s \
 && ln -s /bin/k3s /bin/kubectl \
 && ln -s /bin/k3s /bin/ctr \
 && k3s --version
#install cni plugins manually since we will only install flannel later
RUN set -e -x \
 && mkdir -p /opt/cni/bin \
 && CNI_TGZ="cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz" \
 && case "${ARCH}" in \
      amd64) CNI_SHA256="b98f74a0f8522f0a83867178729c1aa70f2158f90c45a2ca8fa791db1c76b303" ;; \
      arm)   CNI_SHA256="21416880bea0541d78afaf106373d6dbb471edb92c0114fa263494fe4aec8d3b" ;; \
      arm64) CNI_SHA256="56171987d3947707c3563db2f4001bccaf50fd63468611b9f3cbecb1375ee7ec" ;; \
      *) echo "unsupported ARCH for CNI plugins: ${ARCH}" >&2; exit 1 ;; \
    esac \
 && curl -fsSL "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/${CNI_TGZ}" -o "/tmp/${CNI_TGZ}" \
 && { printf '%s  %s\n' "${CNI_SHA256}" "/tmp/${CNI_TGZ}" | sha256sum -c - || { echo "ERROR: CNI plugins checksum verification failed for ${ARCH}. Expected: ${CNI_SHA256}"; exit 1; }; } \
 && tar -xzf "/tmp/${CNI_TGZ}" -C /opt/cni/bin \
 && rm -f "/tmp/${CNI_TGZ}"

COPY scratch/*.tar /var/lib/rancher/k3s/agent/images/
RUN ls -l /var/lib/rancher/k3s/agent/images/
#ADD https://github.com/rancher/k3s/releases/download/${K3S_VERSION}/k3s-airgap-images-${ARCH}.tar /var/lib/rancher/k3s/agent/images/

VOLUME /var/lib/cni
VOLUME /var/lib/kubelet
VOLUME /var/lib/rancher/k3s
VOLUME /var/log

ENTRYPOINT ["k3s"]
CMD ["server"]
