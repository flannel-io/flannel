#!/bin/bash
set -e

echo "### Dry run with input & output files set"
echo "$ ./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt"
! read -d '' EXPECTED <<EOF 
DOCKER_OPT_BIP="--bip=10.1.74.1/24"
DOCKER_OPT_IPMASQ="--ip-masq=true"
DOCKER_OPT_MTU="--mtu=1472"
DOCKER_OPTS=" --bip=10.1.74.1/24 --ip-masq=true --mtu=1472"
EOF
./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt
diff -B -b here.txt <(echo -e "${EXPECTED}")
echo


echo "### Individual vars only (Note DOCKER_OPTS= is missing)"
echo "$ ./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt -i"
! read -d '' EXPECTED <<EOF 
DOCKER_OPT_BIP="--bip=10.1.74.1/24"
DOCKER_OPT_IPMASQ="--ip-masq=true"
DOCKER_OPT_MTU="--mtu=1472"
EOF
./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt -i
diff -B -b here.txt <(echo -e "${EXPECTED}")
echo


echo "### Combined vars only (Note DOCKER_OPT_* vars are missing)"
echo "$ ./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt -c"
! read -d '' EXPECTED <<EOF 
DOCKER_OPTS=" --bip=10.1.74.1/24 --ip-masq=true --mtu=1472"
EOF
./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt -c
diff -B -b here.txt <(echo -e "${EXPECTED}")
echo


echo "### Custom key test (Note DOCKER_OPTS= is substituted by CUSTOM_KEY=)"
echo "$ ./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt -k CUSTOM_KEY"
! read -d '' EXPECTED <<EOF 
DOCKER_OPT_BIP="--bip=10.1.74.1/24"
DOCKER_OPT_IPMASQ="--ip-masq=true"
DOCKER_OPT_MTU="--mtu=1472"
CUSTOM_KEY=" --bip=10.1.74.1/24 --ip-masq=true --mtu=1472"
EOF
./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt -k CUSTOM_KEY
diff -B -b here.txt <(echo -e "${EXPECTED}")
echo


echo "### Ip-masq stripping test (Note DOCKER_OPT_IPMASQ and --ip-masq=true are missing)"
echo "$ ./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt -m"
! read -d '' EXPECTED <<EOF 
DOCKER_OPT_BIP="--bip=10.1.74.1/24"
DOCKER_OPT_MTU="--mtu=1472"
DOCKER_OPTS=" --bip=10.1.74.1/24 --mtu=1472"
EOF
./mk-docker-opts.sh -f ./sample_subnet.env -d here.txt -m
diff -B -b here.txt <(echo -e "${EXPECTED}")

