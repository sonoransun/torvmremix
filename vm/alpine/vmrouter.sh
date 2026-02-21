#!/bin/sh
# Utility script for Tor VM routing
# Source or run directly.

if [ -z $CLIENT_BLOCK_TCP_PORTS ]; then
  CLIENT_BLOCK_TCP_PORTS="445 139 138 137 53 25"
fi
if [ -z $TOR_TRANSPORT ]; then
  TOR_TRANSPORT=9095
fi
if [ -z $TOR_DNSPORT ]; then
  TOR_DNSPORT=9093
fi
if [ -z $LOG_TO ]; then
  LOG_TO=/var/log/vmrouter.log
fi
if [ -z $DOLOG ]; then
  export DOLOG=1
fi
if [ $DOLOG -eq 0 ]; then
  LOG_TO=/dev/null
fi
# user defined targets
if [ -z $trap_tbl ]; then
  trap_tbl="TRAP"
fi
if [ -z $host_filt_tbl ]; then
  host_filt_tbl="HOSTIN"
fi
if [ -z $cli_filt_tbl ]; then
  cli_filt_tbl="CLIIN"
fi
if [ -z $cli_prenat_tbl ]; then
  cli_prenat_tbl="CLIPRE"
fi
if [ -z $cli_postnat_tbl ]; then
  cli_postnat_tbl="CLIPOST"
fi

export FAIL=99
# XXX: right now we don't track error output.

vmr_trapon() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_trapon:">>$LOG_TO 2>&1; fi
  iptables -t filter -I $trap_tbl -j DROP >>$LOG_TO 2>&1
}

vmr_trapoff() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_trapoff:">>$LOG_TO 2>&1; fi
  iptables -t filter --flush $trap_tbl >>$LOG_TO 2>&1
  iptables -t filter -I $trap_tbl -j RETURN >>$LOG_TO 2>&1
}

vmr_init() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_init:">>$LOG_TO 2>&1; fi
  iptables -t filter --flush INPUT >>$LOG_TO 2>&1
  iptables -t filter --flush FORWARD >>$LOG_TO 2>&1
  iptables -t filter --flush OUTPUT >>$LOG_TO 2>&1
  iptables -t nat --flush PREROUTING >>$LOG_TO 2>&1
  iptables -t nat --flush POSTROUTING >>$LOG_TO 2>&1

  # default policy drop
  iptables -t filter -P INPUT DROP >>$LOG_TO 2>&1
  iptables -t filter -P FORWARD DROP >>$LOG_TO 2>&1
  iptables -t filter -P OUTPUT DROP >>$LOG_TO 2>&1

  # trap table is the global on/off switch for traffic
  # use a trap table so that drop can be set
  # as atomic op across input/forward/output.
  iptables -t filter -N $trap_tbl >>$LOG_TO 2>&1
  iptables -t filter -A $trap_tbl -j RETURN >>$LOG_TO 2>&1
  iptables -t filter -I INPUT -j $trap_tbl >>$LOG_TO 2>&1
  iptables -t filter -I FORWARD -j $trap_tbl >>$LOG_TO 2>&1
  iptables -t filter -I OUTPUT -j $trap_tbl >>$LOG_TO 2>&1

  # loopback device is exempt from filtering
  iptables -t filter -I INPUT -i lo -j ACCEPT >>$LOG_TO 2>&1
  iptables -t filter -I OUTPUT -o lo -j ACCEPT >>$LOG_TO 2>&1

  # host filter traffic things to/from the VM
  iptables -t filter -N $host_filt_tbl >>$LOG_TO 2>&1
  iptables -t filter -A INPUT -j $host_filt_tbl >>$LOG_TO 2>&1
  iptables -t filter -A $host_filt_tbl -j RETURN >>$LOG_TO 2>&1

  # client tables for routed traffic
  iptables -t filter -N $cli_filt_tbl >>$LOG_TO 2>&1
  iptables -t filter -A $cli_filt_tbl -j RETURN >>$LOG_TO 2>&1
  iptables -t filter -A FORWARD -j $cli_filt_tbl >>$LOG_TO 2>&1
  iptables -t nat -N $cli_prenat_tbl >>$LOG_TO 2>&1
  iptables -t nat -A PREROUTING -j $cli_prenat_tbl >>$LOG_TO 2>&1
  iptables -t nat -N $cli_postnat_tbl >>$LOG_TO 2>&1
  iptables -t nat -A POSTROUTING -j $cli_postnat_tbl >>$LOG_TO 2>&1
}

vmr_logdrop() {
  # log default drop targets
  iptables -t filter -A INPUT -j LOG >>$LOG_TO 2>&1
  iptables -t filter -A FORWARD -j LOG >>$LOG_TO 2>&1
  iptables -t filter -A OUTPUT -j LOG >>$LOG_TO 2>&1
}

vmr_fwdsetup() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_fwdsetup:">>$LOG_TO 2>&1; fi
  # expects default route interface argument
  if [ -z $1 ]; then
    return $FAIL
  fi
  iptables -t filter -I $cli_filt_tbl -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu >>$LOG_TO 2>&1
  iptables -t filter -I $cli_filt_tbl -m state --state RELATED,ESTABLISHED -j ACCEPT >>$LOG_TO 2>&1
  iptables -t filter -I $cli_filt_tbl -m state --state INVALID -j DROP >>$LOG_TO 2>&1
  for PORTNUM in $CLIENT_BLOCK_TCP_PORTS; do
    iptables -t filter -I $cli_filt_tbl -p tcp --dport $PORTNUM -j DROP >>$LOG_TO 2>&1
  done
  iptables -t nat -I $cli_postnat_tbl -o "$1" -j MASQUERADE >>$LOG_TO 2>&1
  iptables -t filter -I $host_filt_tbl -i "$1" -m state --state ESTABLISHED,RELATED -j ACCEPT >>$LOG_TO 2>&1
  iptables -t filter -I OUTPUT -o "$1" -j ACCEPT >>$LOG_TO 2>&1
  # reset the trap target at top of chain
  iptables -t filter -D OUTPUT -j $trap_tbl >>$LOG_TO 2>&1
  iptables -t filter -I OUTPUT -j $trap_tbl >>$LOG_TO 2>&1
}

vmr_fwdadd() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_fwdadd:">>$LOG_TO 2>&1; fi
  # expects interface to forward for as argument
  if [ -z $1 ]; then
    return $FAIL
  fi
  iptables -t nat -A $cli_prenat_tbl -i "$1" -p tcp -d "$2" -j ACCEPT >>$LOG_TO 2>&1
  iptables -t nat -A $cli_prenat_tbl -i "$1" -p tcp -j REDIRECT --to $TOR_TRANSPORT >>$LOG_TO 2>&1
  iptables -t nat -A $cli_prenat_tbl -i "$1" -p udp --dport 53 -j REDIRECT --to $TOR_DNSPORT >>$LOG_TO 2>&1
  iptables -t nat -A $cli_prenat_tbl -i "$1" -p udp -j DROP >>$LOG_TO 2>&1
  iptables -t filter -I $host_filt_tbl -i "$1" -p udp ! --dport $TOR_DNSPORT -j DROP >>$LOG_TO 2>&1
  iptables -t filter -I OUTPUT -o "$1" -j ACCEPT >>$LOG_TO 2>&1
  # reset the trap target at top of chain
  iptables -t filter -D OUTPUT -j $trap_tbl >>$LOG_TO 2>&1
  iptables -t filter -I OUTPUT -j $trap_tbl >>$LOG_TO 2>&1
}

vmr_fwddel() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_fwddel:">>$LOG_TO 2>&1; fi
  # expects interface to forward for as argument
  if [ -z $1 ]; then
    return $FAIL
  fi
  iptables -t nat -D $cli_prenat_tbl -i "$1" -p tcp -d "$2" -j ACCEPT >>$LOG_TO 2>&1
  iptables -t nat -D $cli_prenat_tbl -i "$1" -p tcp -j REDIRECT --to $TOR_TRANSPORT >>$LOG_TO 2>&1
  iptables -t nat -D $cli_prenat_tbl -i "$1" -p udp --dport 53 -j REDIRECT --to $TOR_DNSPORT >>$LOG_TO 2>&1
  iptables -t nat -D $cli_prenat_tbl -i "$1" -p udp -j DROP >>$LOG_TO 2>&1
  iptables -t filter -D OUTPUT -o "$1" -j ACCEPT >>$LOG_TO 2>&1
}

vmr_opendhcp() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_opendhcp:">>$LOG_TO 2>&1; fi
  # expects dhcp interface as argument
  if [ -z $1 ]; then
    return $FAIL
  fi
  iptables -t filter -I $host_filt_tbl -i "$1" -p udp --dport 67:68 --sport 67:68 -j ACCEPT >>$LOG_TO 2>&1
}

vmr_opentcp() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_opentcp:">>$LOG_TO 2>&1; fi
  iptables -t filter -D $host_filt_tbl -i "$1" -d "$2" -p tcp --dport "$3" -j DROP >>$LOG_TO 2>&1
  iptables -t filter -I $host_filt_tbl -i "$1" -d "$2" -p tcp --dport "$3" -j ACCEPT >>$LOG_TO 2>&1
}

vmr_openudp() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_openudp:">>$LOG_TO 2>&1; fi
  iptables -t filter -D $host_filt_tbl -i "$1" -d "$2" -p udp --dport "$3" -j DROP >>$LOG_TO 2>&1
  iptables -t filter -I $host_filt_tbl -i "$1" -d "$2" -p udp --dport "$3" -j ACCEPT >>$LOG_TO 2>&1
}

vmr_closetcp() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_closetcp:">>$LOG_TO 2>&1; fi
  iptables -t filter -D $host_filt_tbl -i "$1" -d "$2" -p tcp --dport "$3" -j ACCEPT >>$LOG_TO 2>&1
  iptables -t filter -I $host_filt_tbl -i "$1" -d "$2" -p tcp --dport "$3" -j DROP >>$LOG_TO 2>&1
}

vmr_closeudp() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_closeudp:">>$LOG_TO 2>&1; fi
  iptables -t filter -D $host_filt_tbl -i "$1" -d "$2" -p udp --dport "$3" -j ACCEPT >>$LOG_TO 2>&1
  iptables -t filter -I $host_filt_tbl -i "$1" -d "$2" -p udp --dport "$3" -j DROP >>$LOG_TO 2>&1
}

vmr_redirtcp() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_redirtcp:">>$LOG_TO 2>&1; fi
  iptables -t nat -A $cli_prenat_tbl -i "$1" -d "$2" -p tcp --dport "$3" -j REDIRECT --to "$4" >>$LOG_TO 2>&1
}

vmr_undirtcp() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_undirtcp:">>$LOG_TO 2>&1; fi
  iptables -t nat -D $cli_prenat_tbl -i "$1" -d "$2" -p tcp --dport "$3" -j REDIRECT --to "$4" >>$LOG_TO 2>&1
}

vmr_setarp() {
  if [ $DOLOG -eq 1 ]; then echo "vmr_setarp:">>$LOG_TO 2>&1; fi
  # expects interface, ip, mac arguments
  if [ -z $1 ]; then
    return $FAIL
  fi
  if [ -z $2 ]; then
    return $FAIL
  fi
  if [ -z $3 ]; then
    return $FAIL
  fi
  arp -i "$1" -s "$2" "$3" >>$LOG_TO 2>&1
}
