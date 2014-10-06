FROM quay.io/coreos/flannelbox:1.0

MAINTAINER Eugene Yakubovich <eugene.yakubovich@coreos.com>

ADD ./bin/flanneld /opt/bin/

CMD /opt/bin/flanneld
