ARG BASEIMAGE=amd64/alpine:3.24@sha256:d5487879ebd5f0264ca2db5af02584e8d9090403f54cb3abcfcd7a5bcc1e7ebb

FROM ${BASEIMAGE}

RUN apk add --update \
    iperf3 \
  && rm -rf /var/cache/apk/*

EXPOSE 5201

ENTRYPOINT ["/usr/bin/iperf3"]

CMD ["--server","-p","5201"]
