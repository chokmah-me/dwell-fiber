#!/usr/bin/env bash
# wsl_acp_validate.sh -- one-command WSL validation run for the ACP bridge.
#
# Run INSIDE the dwell-fiber repo on the BPF-capable WSL Ubuntu 24.04 guest:
#     cd ~/dwell-fiber && bash test/wsl_acp_validate.sh
#
# Does: git pull, make daemon, prepare benign.tar, (re)start the daemon with
# --use-v3-wip --acp-policy, wait for /metrics, run test/v3_measure.sh, and
# print a summary including the dwell_fiber_v3_acp_phase metric.
#
# Needs: sudo (eBPF), curl, python3. Takes ~10-15 minutes (measurement windows).
set -u
export LC_ALL=C

REPO="${REPO:-$PWD}"
RESULTS_DIR="${RESULTS_DIR:-/tmp/v3-results-acp}"
METRICS_URL="http://localhost:9090/metrics"
DAEMON_LOG="/tmp/daemon-v3b.log"

die() { printf 'FATAL: %s\n' "$*" >&2; exit 1; }
[ -f "$REPO/test/v3_measure.sh" ] || die "run from the dwell-fiber repo root (test/v3_measure.sh not found at $REPO)"
cd "$REPO"

printf '=== 1/5 git pull ===\n'
git pull --ff-only 2>/dev/null || printf 'WARN: git pull failed; continuing with working tree\n'

printf '=== 2/5 make daemon ===\n'
make daemon || die "make daemon failed"

printf '=== 3/5 prepare benign.tar ===\n'
python3 test/bench.py --prepare-tar || die "bench --prepare-tar failed"

printf '=== 4/5 (re)start daemon with --use-v3-wip --acp-policy ===\n'
pkill -f dwell-fiber-daemon 2>/dev/null || true
sleep 2
# shellcheck disable=SC2024
sudo ./bin/dwell-fiber-daemon --use-v3-wip --acp-policy > "$DAEMON_LOG" 2>&1 &
echo $! > /tmp/daemon-v3b.pid
for i in $(seq 1 30); do
    curl -s --max-time 2 "$METRICS_URL" | grep -q '^dwell_fiber_v3_price' && break
    if [ "$i" = 30 ]; then die "daemon metrics not reachable after 30s; see $DAEMON_LOG"; fi
    sleep 1
done
printf 'daemon up. ACP phase metric lines exported: '
curl -s --max-time 2 "$METRICS_URL" | grep -c '^dwell_fiber_v3_acp_phase' || true

printf '=== 5/5 v3_measure.sh (takes ~10-15 min) ===\n'
RESULTS_DIR="$RESULTS_DIR" REPO_ROOT="$REPO" bash test/v3_measure.sh

printf '\n=== ACP summary ===\n'
curl -s --max-time 2 "$METRICS_URL" | grep '^dwell_fiber_v3_acp_phase' \
    || printf '(phase metric not exported -- is --acp-policy on?)\n'
printf 'results: %s/results.json\n' "$RESULTS_DIR"
printf 'daemon log: %s\n' "$DAEMON_LOG"
