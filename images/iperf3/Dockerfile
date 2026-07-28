ARG BASEIMAGE=amd64/alpine:3.24@sha256:a0a65670a97d4d6ffc87eed1175c9c7d699813489e0913d96c7b917d7b51e456

FROM ${BASEIMAGE}

RUN apk add --update \
    iperf3 \
  && rm -rf /var/cache/apk/*

EXPOSE 5201

ENTRYPOINT ["/usr/bin/iperf3"]

CMD ["--server","-p","5201"]
