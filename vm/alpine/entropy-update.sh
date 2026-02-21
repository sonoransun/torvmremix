#!/bin/sh
rndfile="$1"
while true; do
  head -c 512 /dev/urandom > "$rndfile" 2>&1
  chmod 600 "$rndfile" >/dev/null 2>&1
  sleep 60 >/dev/null 2>&1
done
