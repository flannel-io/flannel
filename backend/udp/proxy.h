#ifndef PROXY_H
#define PROXY_H

#include <netinet/in.h>

#ifdef CMD_DEFINE
#	define cmdexport
#else
#	define cmdexport static
#endif

cmdexport const int CMD_SET_ROUTE = 1;
cmdexport const int CMD_DEL_ROUTE = 2;
cmdexport const int CMD_STOP      = 3;

typedef struct command {
	int       cmd;
	in_addr_t dest_net;
	int       dest_net_len;
	in_addr_t next_hop_ip;
	short     next_hop_port;
} command;

void run_proxy(int tun, int sock, int ctl, in_addr_t tun_ip, size_t tun_mtu, int log_errors);

#endif
