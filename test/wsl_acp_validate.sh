#!/usr/bin/env bash
# wsl_acp_validate.sh -- one-command WSL validation run for the ACP bridge.
#
# Run INSIDE the dwell-fiber repo on the BPF-capable WSL Ubuntu 24.04 guest:
#     cd ~/dwell-fiber && bash test/wsl_acp_validate.sh
#
# Does: git pull, make daemon, prepare benign.tar, (re)start the daemon with
# --use-v3-wip [--acp-policy], wait for /metrics, run test/v3_measure.sh, and
# print a summary including the dwell_fiber_v3_acp_phase metric.
#
#   bash test/wsl_acp_validate.sh            # ACP policy ON  (results: /tmp/v3-results-acp)
#   bash test/wsl_acp_validate.sh --control  # ACP policy OFF (results: /tmp/v3-results-ctrl)
#
# The --control arm is the policy-off baseline for isolating the ACP policy's
# marginal effect: same host, same workload, fixed V3 pricing.
#
# Needs: sudo (eBPF), curl, python3. Takes ~10-15 minutes (measurement windows).
set -u
export LC_ALL=C

CONTROL=0
[ "${1:-}" = "--control" ] && CONTROL=1

REPO="${REPO:-$PWD}"
if [ "$CONTROL" = 1 ]; then
    RESULTS_DIR="${RESULTS_DIR:-/tmp/v3-results-ctrl}"
    DAEMON_LOG="/tmp/daemon-v3b-ctrl.log"
    ACP_FLAG=""
else
    RESULTS_DIR="${RESULTS_DIR:-/tmp/v3-results-acp}"
    DAEMON_LOG="/tmp/daemon-v3b.log"
    ACP_FLAG="--acp-policy"
fi
METRICS_URL="http://localhost:9090/metrics"

die() { printf 'FATAL: %s\n' "$*" >&2; exit 1; }
[ -f "$REPO/test/v3_measure.sh" ] || die "run from the dwell-fiber repo root (test/v3_measure.sh not found at $REPO)"
cd "$REPO"

printf '=== 1/5 git pull ===\n'
git pull --ff-only 2>/dev/null || printf 'WARN: git pull failed; continuing with working tree\n'

printf '=== 2/5 make daemon ===\n'
make daemon || die "make daemon failed"

printf '=== 3/5 prepare benign.tar ===\n'
python3 test/bench.py --prepare-tar || die "bench --prepare-tar failed"

if [ "$CONTROL" = 1 ]; then
    printf '=== 4/5 (re)start daemon with --use-v3-wip (CONTROL: no --acp-policy) ===\n'
else
    printf '=== 4/5 (re)start daemon with --use-v3-wip --acp-policy ===\n'
fi
# NOTE: the daemon runs as root (sudo), so killing a previous instance needs sudo too.
sudo pkill -f dwell-fiber-daemon 2>/dev/null || true
sleep 2
if curl -s --max-time 2 "$METRICS_URL" | grep -q '^dwell_fiber_v3_price'; then
    die "port 9090 still serving after pkill -- is another dwell-fiber-daemon running? (sudo ss -ltnp | grep 9090)"
fi
# shellcheck disable=SC2024
# shellcheck disable=SC2086
sudo ./bin/dwell-fiber-daemon --use-v3-wip $ACP_FLAG > "$DAEMON_LOG" 2>&1 &
echo $! > /tmp/daemon-v3b.pid
for i in $(seq 1 30); do
    # NOTE: check the V2 metric, not the V3 one -- if BPF fails to load the
    # daemon falls back to simulation mode and the V3 controller never starts.
    curl -s --max-time 2 "$METRICS_URL" | grep -q '^dwell_fiber_price' && break
    if [ "$i" = 30 ]; then die "daemon metrics not reachable after 30s; see $DAEMON_LOG"; fi
    sleep 1
done
# Warn loudly if we are in simulation mode: V3/ACP measurements are meaningless there.
if ! curl -s --max-time 2 "$METRICS_URL" | grep -q '^dwell_fiber_v3_price'; then
    printf '⚠️  WARNING: dwell_fiber_v3_price not exported -- daemon is in SIMULATION mode (BPF failed to load).\n'
    printf '⚠️  V3/ACP measurements from this run are NOT trustworthy. Check "Failed to load BPF" in %s.\n' "$DAEMON_LOG"
fi
printf 'daemon up. ACP phase metric lines exported: '
curl -s --max-time 2 "$METRICS_URL" | grep -c '^dwell_fiber_v3_acp_phase' || true

printf '=== 5/5 v3_measure.sh (takes ~10-15 min) ===\n'
RESULTS_DIR="$RESULTS_DIR" REPO_ROOT="$REPO" bash test/v3_measure.sh

printf '\n=== ACP summary ===\n'
if [ "$CONTROL" = 1 ]; then
    if curl -s --max-time 2 "$METRICS_URL" | grep -q '^dwell_fiber_v3_acp_phase'; then
        printf 'UNEXPECTED: phase metric exported in control run (policy should be off)\n'
    else
        printf '(control run: no phase metric, as expected -- fixed V3 pricing)\n'
    fi
else
    curl -s --max-time 2 "$METRICS_URL" | grep '^dwell_fiber_v3_acp_phase' \
        || printf '(phase metric not exported -- is --acp-policy on?)\n'
fi
printf 'results: %s/results.json\n' "$RESULTS_DIR"
printf 'daemon log: %s\n' "$DAEMON_LOG"
