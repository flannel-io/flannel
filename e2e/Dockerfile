# Install the all-in-one binary so we can copy our run-time images into the image
# which helps avoid pulling them when running e2e tests.
ARG SLES="registry.suse.com/suse/sle15:15.3"
FROM ${SLES} AS k3s
ARG ARCH
ARG K3S_VERSION="v1.23.13+k3s1"
ARG CNI_VERSION="v1.1.1" 
RUN set -x \
 && zypper -n in \
    ca-certificates \
    curl \
    tar gzip\
    iptables \
    iproute2 \
    iputils \
 && if [ "${ARCH?required}" != "amd64" ]; then \
        K3S_SUFFIX="-${ARCH}"; \
    fi \
 && curl -fsSL "https://github.com/rancher/k3s/releases/download/${K3S_VERSION}/k3s${K3S_SUFFIX}" > /bin/k3s \
 && chmod +x /bin/k3s \
 && ln -s /bin/k3s /bin/kubectl \
 && ln -s /bin/k3s /bin/ctr \
 && k3s --version
#install cni plugins manually since we will only install flannel later
RUN mkdir -p /opt/cni/bin \
 && curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz
  
COPY scratch/*.tar /var/lib/rancher/k3s/agent/images/
RUN ls -l /var/lib/rancher/k3s/agent/images/
#ADD https://github.com/rancher/k3s/releases/download/${K3S_VERSION}/k3s-airgap-images-${ARCH}.tar /var/lib/rancher/k3s/agent/images/

VOLUME /var/lib/cni
VOLUME /var/lib/kubelet
VOLUME /var/lib/rancher/k3s
VOLUME /var/log

ENTRYPOINT ["k3s"]
CMD ["server"]
