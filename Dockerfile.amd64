FROM busybox:1.25.0-glibc

MAINTAINER Tom Denham <tom@tigera.io>

COPY dist/flanneld-amd64 /opt/bin/flanneld
COPY dist/iptables-amd64 /usr/local/bin/iptables
COPY dist/mk-docker-opts.sh /opt/bin/
COPY dist/libpthread.so.0-amd64 /lib/libpthread.so.0
CMD ["/opt/bin/flanneld"]

