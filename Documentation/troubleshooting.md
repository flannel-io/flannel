### Firewalls
When using `udp` backend, flannel uses UDP port 8285 for sending encapsulated packets.
When using `vxlan` backend, kernel uses UDP port 8472 for sending encapsulated packets.
Make sure that your firewall rules allow this traffic for all hosts participating in the overlay network.
