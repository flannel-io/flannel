ARG XX_IMAGE=tonistiigi/xx:1.8.0
ARG GO_IMAGE=golang:alpine3.22
ARG RUN_IMAGE=alpine:3.22.2

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
COPY cmd /build/cmd
WORKDIR /build
RUN mkdir dist
RUN go mod download

ARG TARGETPLATFORM
ARG TAG
RUN xx-info env
RUN export GOOS=$(xx-info os) &&\
    export GOARCH=$(xx-info arch) &&\
    export ARCH=$(xx-info arch) &&\
    make dist/flanneld
RUN export GOOS=$(xx-info os) &&\
    export GOARCH=$(xx-info arch) &&\
    CGO_ENABLED=0 go build -o dist/install-conf ./cmd/install_conf/
RUN git clone https://github.com/kubernetes-sigs/iptables-wrappers.git /iptables-wrapper
WORKDIR /iptables-wrapper
RUN git checkout bfef9e5087a198b50a4124bb9ce9d2c7c99025dd
RUN export GOOS=$(xx-info os) &&\
    export GOARCH=$(xx-info arch) &&\
    export ARCH=$(xx-info arch) &&\
    make build

FROM ${RUN_IMAGE}
RUN apk update && apk upgrade
RUN apk add --no-cache iproute2 ca-certificates nftables iptables strongswan iptables-legacy && update-ca-certificates
RUN apk add wireguard-tools --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/community
COPY --from=flannel-builder /build/dist/flanneld /opt/bin/flanneld
COPY --from=flannel-builder /build/dist/install-conf /opt/bin/install-conf
COPY dist/mk-docker-opts.sh /opt/bin/
COPY --from=flannel-builder /iptables-wrapper/bin/iptables-wrapper /usr/sbin
# check manually that iptables-legacy and iptables-nft are present since
# iptables-wrapper-installer.sh sanity check doesn't work for multi-arch build
RUN which iptables-legacy && which iptables-nft
RUN /usr/sbin/iptables-wrapper install


ENTRYPOINT ["/opt/bin/flanneld"]

