#!/usr/bin/env bash
# v3_measure.sh — self-contained V3 budget-experiment measurement
#
# Runs the benign + intermittent windows with concurrent poller + bench,
# drains between windows, and writes results to $RESULTS_DIR/results.json.
#
# Pre-requisites:
#   - Daemon running: sudo ./bin/dwell-fiber-daemon --use-v3-wip
#   - Benign tar pre-generated: python3 test/bench.py --prepare-tar
#
# Usage:
#   cd ~/dwell-fiber && bash test/v3_measure.sh
#
# On ambient: the guest has ~30s cadence (unknown) bursts (UFM 200-679/s,
# TBW 0).  Price never reaches literal 0.  This script accepts the floor
# (50.0) as sufficiently drained, and the calibration step uses
# P_b_eff = max(P_b, ambient_ceiling) to account for it.

set -u
export LC_ALL=C

# ---- configuration ----
RESULTS_DIR="${RESULTS_DIR:-/tmp/v3-results}"
METRICS_URL="${METRICS_URL:-http://localhost:9090/metrics}"
FLOOR_THRESHOLD="${FLOOR_THRESHOLD:-50.0}"
FLOOR_TIMEOUT_S="${FLOOR_TIMEOUT_S:-120}"
POLLER_BENIGN_DURATION=180
POLLER_INTERMITTENT_DURATION=300
REPO_ROOT="${REPO_ROOT:-$HOME/dwell-fiber}"

mkdir -p "$RESULTS_DIR"

die() { printf 'FATAL: %s\n' "$*" >&2; exit 1; }

# ---- helpers ----

get_price() {
    curl -s --max-time 2 "$METRICS_URL" \
        | grep '^dwell_fiber_v3_price' \
        | tr -s ' ' \
        | cut -d' ' -f2
    # returns empty when daemon is down; caller handles default
}

price_lt() {
    # exit 0 when float $1 < float $2, else exit 1
    python3 -c \
        "import sys; sys.exit(0 if float(sys.argv[1]) < float(sys.argv[2]) else 1)" \
        "$1" "$2" 2>/dev/null
}

ambient_ceiling_from_log() {
    set +e
    grep -oa 'price=[0-9.]*' /tmp/daemon-v3b.log 2>/dev/null \
        | cut -d= -f2 \
        | sort -n \
        | tail -1
    set -e
}

# ---- drain ----

drain_floor() {
    local tag="$1" t0
    t0=$(date +%s)
    printf '=== drain-floor (%s)  threshold=%s  timeout=%ss ===\n' \
        "$tag" "$FLOOR_THRESHOLD" "$FLOOR_TIMEOUT_S"

    while true; do
        local elapsed p
        elapsed=$(($(date +%s) - t0))
        if [ "$elapsed" -ge "$FLOOR_TIMEOUT_S" ]; then
            printf 'drain-floor: timeout at %ss — proceeding\n' "$elapsed"
            break
        fi

        p=$(get_price)
        p="${p:-0}"
        printf '  +%ss  price=%s\n' "$elapsed" "$p"

        if [ "$p" = "0" ]; then
            printf 'drain-floor: price hit literal 0 at %ss\n' "$elapsed"
            break
        fi
        if price_lt "$p" "$FLOOR_THRESHOLD"; then
            printf 'drain-floor: price %s < %s at %ss\n' \
                "$p" "$FLOOR_THRESHOLD" "$elapsed"
            break
        fi
        sleep 15
    done
}

# ---- measurement ----

measure_window() {
    local scenario="$1" duration="$2"
    local poller_json="$RESULTS_DIR/poller-${scenario}.json"
    local peak_file="$RESULTS_DIR/peak-${scenario}.txt"

    printf '=== measure %s (poller %ss) ===\n' "$scenario" "$duration"
    cd "$REPO_ROOT"

    # ---- poller (background) ----
    printf '  starting poller …\n'
    python3 test/calibrate_v3.py --from-metrics --duration-s "$duration" \
        > "$poller_json" 2>&1 &
    local poller_pid=$!

    sleep 2   # small head start so poller captures the whole bench window

    # ---- bench (foreground) ----
    printf '  running bench …\n'
    set +e
    python3 test/bench.py --scenario "$scenario" \
        --out "$RESULTS_DIR/bench-${scenario}3.md"
    local bench_rc=$?
    set -e

    # ---- wait for poller ----
    printf '  bench exit=%s  waiting for poller (pid %s) …\n' \
        "$bench_rc" "$poller_pid"
    wait "$poller_pid" || true

    # ---- extract peak ----
    # calibrate_v3.py --from-metrics prints a banner line before the JSON body;
    # parse from the first '{' so the banner cannot break the read.
    local peak
    set +e
    peak=$(python3 -c \
        "import json,sys; t=open(sys.argv[1]).read(); print(json.loads(t[t.find('{'):])['peak_price'])" \
        "$poller_json" 2>/dev/null)
    set -e
    if [ -z "$peak" ]; then
        peak=0
        printf '  WARN: could not parse poller JSON (%s); tail of file:\n' \
            "$poller_json" >&2
        tail -3 "$poller_json" >&2
    fi
    printf '%s\n' "$peak" > "$peak_file"
    printf '  peak_price=%s\n' "$peak"
}

# ---- pre-flight ----

check_daemon() {
    local ok
    set +e
    ok=$(curl -s --max-time 2 "$METRICS_URL" 2>/dev/null | grep -c 'dwell_fiber_v3_price')
    set -e
    [ "${ok:-0}" -gt 0 ] || die "daemon ${METRICS_URL} unreachable"
    printf 'pre-flight: daemon reachable at %s\n' "$METRICS_URL"
}

# ======== main ========
printf '=== V3 budget experiment  %s ===\n' "$(date -Iseconds)"
printf 'repo=%s  results=%s\n' "$REPO_ROOT" "$RESULTS_DIR"

check_daemon

P_B=""
P_I=""

# 1 — benign window
drain_floor benign
measure_window benign "$POLLER_BENIGN_DURATION"
P_B=$(cat "$RESULTS_DIR/peak-benign.txt")
printf 'P_b (benign window peak) = %s\n' "$P_B"

# 2 — intermittent window
drain_floor intermittent
measure_window intermittent "$POLLER_INTERMITTENT_DURATION"
P_I=$(cat "$RESULTS_DIR/peak-intermittent.txt")
printf 'P_i (intermittent window peak) = %s\n' "$P_I"

# 3 — ambient ceiling from daemon log
AMBIENT=$(ambient_ceiling_from_log)
AMBIENT="${AMBIENT:-0}"
printf 'ambient_ceiling (from daemon log) = %s\n' "$AMBIENT"

# 4 — write results
printf '{\n  "P_b": %s,\n  "P_i": %s,\n  "ambient_ceiling_log": %s,\n  "date": "%s",\n  "comment": "Ambient ~30s bursts (unknown PIDs, UFM 200-679/s, TBW 0) prevent drain to literal 0. P_b_eff = max(P_b, ambient_ceiling_log)."\n}\n' \
    "$P_B" "$P_I" "$AMBIENT" "$(date -Iseconds)" \
    > "$RESULTS_DIR/results.json"

printf '\n=== results ===\n'
cat "$RESULTS_DIR/results.json"
printf '\n=== done %s ===\n' "$(date -Iseconds)"
