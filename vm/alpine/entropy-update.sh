#!/bin/sh
rndfile="$1"
# Validate path: must start with /home/system/ and contain no ..
case "$rndfile" in
  /home/system/*) ;;
  *) echo "entropy-update: invalid path" >&2; exit 1 ;;
esac
case "$rndfile" in
  *..*) echo "entropy-update: path traversal rejected" >&2; exit 1 ;;
esac
while true; do
  head -c 512 /dev/urandom > "$rndfile" 2>&1
  chmod 600 "$rndfile" >/dev/null 2>&1
  sleep 60 >/dev/null 2>&1
done
