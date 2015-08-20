#!/bin/bash

set -ex

function setup() {
    ## iptables
    iptables -D INPUT -p udp -m multiport --dports 4789 -m comment --comment "001 vxlan incoming" -j ACCEPT || true
    lineno=$(iptables -nvL INPUT --line-numbers | grep "state RELATED,ESTABLISHED" | awk '{print $1}')
    iptables -I INPUT $lineno -p udp -m multiport --dports 4789 -m comment --comment "001 vxlan incoming" -j ACCEPT
}

setup
