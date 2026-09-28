#!/usr/bin/env bash
# wsl_ambient_hunt.sh -- catch the ambient open-storm bursters with forensics.
#
# The ambient contaminant (UFM 200-679/s, TBW ~0, ~30 s cadence, unknown "(9)"
# comms) is short-lived: each burst arrives on a fresh PID. This script watches
# the V3 daemon log for high-UFM/low-TBW samples and, for each candidate PID,
# immediately captures cmdline, PPid, parent cmdline, cwd, and fd count --
# plus continuous `ps` snapshots so dead PIDs can be resolved post-hoc.
#
# Usage: bash test/wsl_ambient_hunt.sh [daemon-log] [duration-secs]
#   daemon-log defaults to /tmp/daemon-v3b.log (the ACP arm of
#   test/wsl_acp_validate.sh); duration defaults to 240 s (~8 burst periods).
#
# Requires: the V3 daemon running (any arm). Read-only w.r.t. the system
# except for files under /tmp/ambient-hunt-*.
set -u

DAEMON_LOG="${1:-/tmp/daemon-v3b.log}"
DURATION="${2:-240}"
OUTDIR="/tmp/ambient-hunt-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUTDIR"
echo "outdir: $OUTDIR  (log: $DAEMON_LOG, duration: ${DURATION}s)"

if [ ! -f "$DAEMON_LOG" ]; then
  echo "daemon log not found: $DAEMON_LOG -- is the V3 daemon running?" >&2
  echo "(start one via: bash test/wsl_acp_validate.sh, then re-run this)" >&2
  exit 1
fi

# 1. Background ps snapshots (1/s) for post-hoc resolution of dead PIDs.
(
  while true; do
    printf '=== %s ===\n' "$(date +%s)"
    ps -eo pid,ppid,etimes,comm,args 2>/dev/null
    sleep 1
  done > "$OUTDIR/ps-snap.log" 2>&1
) &
PSPID=$!

capture_pid() {
  local pid=$1
  [ -f "$OUTDIR/seen-$pid" ] && return 0   # one capture per PID
  touch "$OUTDIR/seen-$pid"
  local f="$OUTDIR/pid-$pid.txt"
  {
    echo "--- $(date -Is) PID=$pid"
    printf 'cmdline: '; tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null; echo
    grep -E '^(PPid|Name|State)' "/proc/$pid/status" 2>/dev/null
    local ppid
    ppid=$(awk '/^PPid/{print $2}' "/proc/$pid/status" 2>/dev/null)
    if [ -n "$ppid" ]; then
      printf 'parent cmdline: '; tr '\0' ' ' < "/proc/$ppid/cmdline" 2>/dev/null; echo
      printf 'parent comm: '; cat "/proc/$ppid/comm" 2>/dev/null; echo
    fi
    echo "cwd: $(readlink "/proc/$pid/cwd" 2>/dev/null)"
    echo "fd count: $(ls "/proc/$pid/fd" 2>/dev/null | wc -l)"
  } > "$f" 2>&1
  echo "captured PID $pid -> $f"
}

# 2. Tail the daemon log for high-UFM / low-TBW candidates.
#    Line shape: [V3] High I/O pressure: PID=2382 ((9)) tier=T2 TBW=0.0MB/s UFM=679/s ...
tail -n0 -F "$DAEMON_LOG" 2>/dev/null | while read -r line; do
  case "$line" in
    *"[V3] High I/O pressure:"*) ;;
    *) continue ;;
  esac
  pid=$(printf '%s' "$line" | grep -oP 'PID=\K[0-9]+' | head -1)
  ufm=$(printf '%s' "$line" | grep -oP 'UFM=\K[0-9]+' | head -1)
  tbw=$(printf '%s' "$line" | grep -oP 'TBW=\K[0-9.]+' | head -1)
  [ -z "$pid" ] && continue
  # Ambient signature: enumeration-rate opens, ~zero writes.
  if [ "${ufm:-0}" -ge 150 ] && [ "${tbw%.*}" -eq 0 ]; then
    capture_pid "$pid"
  fi
done &
TAILPID=$!

sleep "$DURATION"
kill "$TAILPID" "$PSPID" 2>/dev/null
wait 2>/dev/null

# 3. Summary.
echo
echo "=== ambient hunt summary ==="
echo "candidate PIDs captured: $(ls "$OUTDIR"/pid-*.txt 2>/dev/null | wc -l)"
for f in "$OUTDIR"/pid-*.txt; do
  [ -f "$f" ] || continue
  pid=$(basename "$f" .txt); pid=${pid#pid-}
  echo "--- PID $pid"
  grep -E '^(cmdline|PPid|Name|parent cmdline|parent comm|cwd)' "$f"
  # post-hoc: find this PID in the ps snapshots (may reveal args even if dead at capture)
  grep -m1 -E "^[[:space:]]*$pid[[:space:]]" "$OUTDIR/ps-snap.log" | head -1 | sed 's/^/  ps-snap: /'
done
echo
echo "Full output in $OUTDIR -- paste pid-*.txt (or the summary above) back for analysis."
