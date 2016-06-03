#!/bin/bash
set -e

if [ $# -ne 1 ]; then
	echo "Usage: $0 tag" >/dev/stderr
	exit 1
fi

tag=$1

tgt=$(mktemp -d)

# Build flannel inside 

#docker run -v `pwd`/../:/opt/flannel -i -t golang:1.4.2 /bin/bash -c "cd /opt/flannel && ./build && wget https://github.com/strongswan/strongswan/archive/5.3.3rc1.tar.gz" 

docker run -v `pwd`/../:/opt/flannel -i -t golang:1.4.2 /bin/bash -c "cd /opt/flannel && ./build && apt-get update && apt-get install -y libgmp3-dev autoconf automake git libtool pkg-config gettext perl python flex bison gperf && wget https://github.com/strongswan/strongswan/archive/5.3.3rc1.tar.gz && tar -zxvf 5.3.3rc1.tar.gz && cd strongswan-5.3.3rc1 && ./autogen.sh && ./configure --prefix=/opt/flannel --sysconfdir=/opt/flannel --enable-vici --disable-attr --disable-constraints --disable-dnskey --disable-fips-prf --disable-ikev2 --disable-md5 --disable-pgp --disable-pem --disable-pkcs1 --disable-pkcs7 --disable-pkcs8 --disable-pkcs12 --disable-pki --disable-pubkey --disable-rc2 --disable-resolve --disable-revocation --disable-scepclient --disable-stroke --disable-updown --disable-x509 --disable-xauth-generic && make && make install" 

# Generate Dockerfile into target tmp dir
cat <<DF >${tgt}/Dockerfile
FROM quay.io/mohdahmad/flannelbox:0.7
MAINTAINER Eugene Yakubovich <eugene.yakubovich@coreos.com>
ADD ./flanneld /opt/bin/
ADD ./mk-docker-opts.sh /opt/bin/
ADD ./lib /opt/flannel/lib
ADD ./libexec /opt/flannel/libexec
ADD ./share /opt/flannel/share
ADD ./strongswan.d /opt/flannel/strongswan.d
ADD ./strongswan.conf /opt/flannel/
CMD /opt/bin/flanneld
DF

echo -e 'charon {\n	load_modular = yes \n	plugins {\n		include strongswan.d/charon/*.conf \n	}\n	filelog {\n		stderr{\n		ike = 3\n		knl = 2\n		chd = 2\n		net = 2\n		cfg = 2\n		}\n	}\n	group  = "root"\n	 user = "root"\n }\n include strongswan.d/charon/*.conf' > ../strongswan.conf 

# Copy artifcats into target dir and build the image
cp ../bin/flanneld $tgt
cp ./mk-docker-opts.sh $tgt
cp -R ../lib $tgt
cp -R ../libexec $tgt
cp -R ../share $tgt
cp -R ../strongswan.d $tgt
cp ../strongswan.conf $tgt


docker build -t quay.io/mohdahmad/flannel:${tag} $tgt

rm -rf $tgt
rm -rf ../5.3.3rc1.tar.gz
rm -rf ../strongswan-5.3.3rc1
rm -rf ../strongswan.conf
rm -rf ../strongswan.d
rm -rf ../share
rm -rf ../lib
rm -rf ../libexec
