ARG XX_IMAGE=tonistiigi/xx:1.6.1
ARG GO_IMAGE=golang:alpine3.22
ARG RUN_IMAGE=alpine:3.22.0

FROM --platform=$BUILDPLATFORM ${XX_IMAGE}  AS xx

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS base-builder
# copy xx scripts to your build stage
COPY --from=xx / /
RUN apk add bash file make git clang lld patch linux-headers
ARG TARGETPLATFORM
RUN set -x && \
    xx-apk --no-cache add gcc musl-dev 

FROM --platform=$BUILDPLATFORM base-builder AS flannel-builder
COPY Makefile go.mod go.sum *.go /build/
COPY pkg /build/pkg
WORKDIR /build
RUN mkdir dist
RUN go mod download

ARG TARGETPLATFORM
RUN xx-info env
RUN export GOOS=$(xx-info os) &&\
    export GOARCH=$(xx-info arch) &&\
    export ARCH=$(xx-info arch) &&\
    make dist/flanneld
RUN git clone https://github.com/kubernetes-sigs/iptables-wrappers.git /iptables-wrapper
WORKDIR /iptables-wrapper
RUN git checkout 5792812d9e5a5bb7f22d79d557bbfeece253343d
RUN export GOOS=$(xx-info os) &&\
    export GOARCH=$(xx-info arch) &&\
    export ARCH=$(xx-info arch) &&\
    make build

FROM ${RUN_IMAGE}
RUN apk update && apk upgrade
RUN apk add --no-cache iproute2 ca-certificates nftables iptables strongswan iptables-legacy && update-ca-certificates
RUN apk add wireguard-tools --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/community
COPY --from=flannel-builder /build/dist/flanneld /opt/bin/flanneld
COPY dist/mk-docker-opts.sh /opt/bin/
COPY --from=flannel-builder /iptables-wrapper/iptables-wrapper-installer.sh /
COPY --from=flannel-builder /iptables-wrapper/bin/iptables-wrapper /
# check manually that iptables-legacy and iptables-nft are present since
# iptables-wrapper-installer.sh sanity check doesn't work for multi-arch build
RUN which iptables-legacy && which iptables-nft
RUN /iptables-wrapper-installer.sh --no-sanity-check


ENTRYPOINT ["/opt/bin/flanneld"]

