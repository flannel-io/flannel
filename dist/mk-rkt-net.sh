#!/bin/bash

usage() {
	echo "$0 [-f FLANNEL-ENV-FILE] [-n NET-NAME] [-r ROUTES] RKT-NETCONF-FILE

Generate Rocket network file based on flannel env file
OPTIONS:
	-f	Path to flannel env file. Defaults to /run/flannel/subnet.env
	-n	Rocket network name. Defaults to foo RKT-NETCONF-FILE is 10-foo.conf
	-r	Additional routes to add specified as JSON list: e.g. ["10.1.0.0/16", "10.2.3.0/24"]
	-m  Turn on IP masquerade (useful if overriding default net). Defaults to false

ARG:
	Path to Rocket net file to write to. Defaults to /run/docker_opts.env
" >/dev/stderr 

	exit 1
}

flannel_env="/run/flannel/subnet.env"
routes="[]"
ipmasq="false"

while getopts "f:n:r:m" opt; do
	case $opt in
		f)
			flannel_env=$OPTARG
			;;
		n)
			netname=$OPTARG
			;;
		r)
			routes=$OPTARG
			;;
		m)
			ipmasq="true"
			;;
		\?)
			usage
			;;
	esac
done

shift $((OPTIND-1))

netconf=$1
if [ -z "$netconf" ]; then
	echo "RKT-NETCONF-FILE missing" >/dev/stderr
	usage
fi

if [ -z "$netname" ]; then
	regex='[0-9]+-(.*)\.conf'
	[[ $netconf =~ $regex ]]
	netname=${BASH_REMATCH[1]}

	if [ -z "$netname" ]; then
		echo "-n NETNAME not provided and unable to extract from $netconf" >/dev/stderr
		exit 1
	fi
fi

if [ -f "$flannel_env" ]; then
	source $flannel_env
fi

mkdir -p /etc/rkt/net.d

cat >/etc/rkt/net.d/$netconf <<EOF
{
	"name": "$netname",
	"type": "veth",
	"mtu": $FLANNEL_MTU,
	"ipMasq": $ipmasq,
	"ipam": {
		"type": "static-ptp",
		"subnet": "$FLANNEL_SUBNET",
		"routes": $routes
	}
}
EOF
