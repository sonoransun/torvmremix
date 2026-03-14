#!/bin/sh
# Browser VM iptables rules.
# Restricts all traffic to Tor VM SOCKS5 and DNS ports only.

# Default DROP everything.
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT DROP

# Allow loopback (required for Xvfb and IPC).
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT

# Allow established/related connections.
iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# Allow SOCKS5 to Tor VM.
iptables -A OUTPUT -p tcp -d "$TORIP" --dport "$SOCKSPORT" -j ACCEPT

# Allow DNS to Tor VM.
iptables -A OUTPUT -p tcp -d "$TORIP" --dport "$DNSPORT" -j ACCEPT

# Allow DHCP (QEMU user-mode networking uses DHCP).
iptables -A OUTPUT -p udp --dport 67 -j ACCEPT
iptables -A INPUT -p udp --sport 67 -j ACCEPT

# Log and drop everything else.
iptables -A OUTPUT -m limit --limit 5/min -j LOG --log-prefix "BROWSER_BLOCKED: " --log-level 4
iptables -A OUTPUT -j DROP
iptables -A INPUT -j DROP

# Disable IPv6 completely.
ip6tables -P INPUT DROP
ip6tables -P FORWARD DROP
ip6tables -P OUTPUT DROP
ip6tables -A INPUT -i lo -j ACCEPT
ip6tables -A OUTPUT -o lo -j ACCEPT
