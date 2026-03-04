#!/bin/sh
# entropy-mix.sh - Entropy health monitor and mixer for TorVM
# Periodically checks entropy pool health and mixes additional entropy
# from available sources. Runs as a background daemon.

INTERVAL=30
MIN_ENTROPY=256
LOG_TAG="entropy-mix"

log() {
  echo "$LOG_TAG: $1" >> /var/log/entropy-mix.log 2>&1
}

# Wait for system initialization to complete.
sleep 5

log "starting (interval=${INTERVAL}s, min_entropy=${MIN_ENTROPY})"

# Detect available entropy sources on startup.
SOURCES=""
[ -c /dev/hwrng ] && SOURCES="$SOURCES hwrng"
[ -c /dev/ttyS1 ] && SOURCES="$SOURCES serial"
grep -q rdrand /proc/cpuinfo 2>/dev/null && SOURCES="$SOURCES rdrand"
grep -q padlock /proc/crypto 2>/dev/null && SOURCES="$SOURCES padlock"
pidof rngd >/dev/null 2>&1 && SOURCES="$SOURCES rngd"
pidof haveged >/dev/null 2>&1 && SOURCES="$SOURCES haveged"

log "detected sources:$SOURCES"

while true; do
  # Read current entropy available (in bits).
  AVAIL=$(cat /proc/sys/kernel/random/entropy_avail 2>/dev/null || echo 0)

  if [ "$AVAIL" -lt "$MIN_ENTROPY" ]; then
    log "low entropy: ${AVAIL} bits (threshold: ${MIN_ENTROPY})"

    # Mix from /dev/hwrng if available.
    if [ -c /dev/hwrng ]; then
      timeout 2 head -c 512 /dev/hwrng > /dev/urandom 2>/dev/null
    fi

    # Mix from timing jitter: measure nanosecond-resolution timestamps
    # and kernel interrupt/timer state as a last-resort entropy source.
    {
      date +%N 2>/dev/null
      cat /proc/interrupts 2>/dev/null
      cat /proc/timer_list 2>/dev/null | head -50
    } | sha256sum > /dev/urandom 2>/dev/null

    # Re-check after mixing.
    AVAIL_AFTER=$(cat /proc/sys/kernel/random/entropy_avail 2>/dev/null || echo 0)
    log "after mixing: ${AVAIL_AFTER} bits"

    # If entropy is critically low and daemons are dead, restart them.
    if [ "$AVAIL_AFTER" -lt 64 ]; then
      if ! pidof rngd >/dev/null 2>&1; then
        if command -v rngd >/dev/null 2>&1; then
          rngd -r /dev/hwrng 2>/dev/null || rngd 2>/dev/null
          log "restarted rngd (entropy critically low)"
        fi
      fi
      if ! pidof haveged >/dev/null 2>&1; then
        if command -v haveged >/dev/null 2>&1; then
          haveged -w 1024 -v 0 2>/dev/null
          log "restarted haveged (entropy critically low)"
        fi
      fi
    fi
  fi

  sleep "$INTERVAL"
done
