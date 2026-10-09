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
# On ambient: the guest showed ~30s cadence (unknown) bursts (UFM 200-679/s,
# TBW 0) on 2026-09-25; none have been seen since ~03:11 on 2026-09-28.
# Ambient is now measured honestly: a 60s idle poller window before the
# benches, whose peak is the ambient ceiling. The calibration step uses
# P_b_eff = max(P_b, ambient_ceiling_idle) to account for it.

set -u
export LC_ALL=C

# ---- configuration ----
RESULTS_DIR="${RESULTS_DIR:-/tmp/v3-results}"
METRICS_URL="${METRICS_URL:-http://localhost:9090/metrics}"
FLOOR_THRESHOLD="${FLOOR_THRESHOLD:-50.0}"
FLOOR_TIMEOUT_S="${FLOOR_TIMEOUT_S:-120}"
POLLER_BENIGN_DURATION=180
POLLER_INTERMITTENT_DURATION=300
# Paced files/s for the intermittent bench (0 = unpaced legacy). 350
# replicates the 2026-09-25 calibration conditions; the unpaced bench's rate
# is host-speed-dependent (P_i=681 on 09-25 vs P_i=0.0 on 09-28, same script).
INTERMITTENT_RATE="${INTERMITTENT_RATE:-350}"
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

# ---- peak extraction (shared) ----
# calibrate_v3.py --from-metrics prints a banner line before the JSON body;
# parse from the first '{' so the banner cannot break the read.
extract_peak() {
    local poller_json="$1" peak_file="$2"
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

# Peak over poller samples where the ambient-storm gauge read 0. Falls back to
# the plain peak when the poller JSON has no series/ambient_storm (older
# daemon or parse failure) so calibration never silently drops data.
extract_peak_nonambient() {
    local poller_json="$1" peak_file="$2"
    local peak
    set +e
    peak=$(python3 -c \
        "import json,sys
t=open(sys.argv[1]).read()
d=json.loads(t[t.find('{'):])
s=d.get('series') or []
vals=[x.get('price',0) for x in s if x.get('ambient_storm',0)==0]
print(max(vals) if vals else d.get('peak_price',0))" \
        "$poller_json" 2>/dev/null)
    set -e
    if [ -z "$peak" ]; then
        peak=$(cat "${peak_file/-nonambient/}" 2>/dev/null || printf '0')
        printf '  WARN: nonambient peak parse failed; using plain peak\n' >&2
    fi
    printf '%s\n' "$peak" > "$peak_file"
    printf '  peak_price_nonambient=%s\n' "$peak"
}

# ---- idle ambient window (no bench) ----
# The true ambient ceiling: peak V3 price while the machine is otherwise
# idle. The old approach (max price= over the whole daemon log) measured the
# bench itself, not ambient.
measure_idle() {
    local duration="$1"
    local poller_json="$RESULTS_DIR/poller-ambient.json"
    local peak_file="$RESULTS_DIR/peak-ambient.txt"

    printf '=== measure ambient (idle poller %ss, no bench) ===\n' "$duration"
    cd "$REPO_ROOT"

    printf '  starting poller …\n'
    python3 test/calibrate_v3.py --from-metrics --duration-s "$duration" \
        > "$poller_json" 2>&1 &
    local poller_pid=$!

    printf '  waiting for poller (pid %s) …\n' "$poller_pid"
    wait "$poller_pid" || true

    extract_peak "$poller_json" "$peak_file"
}

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
        --intermittent-rate "$INTERMITTENT_RATE" \
        --out "$RESULTS_DIR/bench-${scenario}3.md"
    local bench_rc=$?
    set -e

    # ---- wait for poller ----
    printf '  bench exit=%s  waiting for poller (pid %s) …\n' \
        "$bench_rc" "$poller_pid"
    wait "$poller_pid" || true

    extract_peak "$poller_json" "$peak_file"
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
AMBIENT=""
P_B_CLEAN=""
P_I_CLEAN=""

# 0 — ambient idle window (true ambient ceiling; nothing running)
AMBIENT_DURATION="${AMBIENT_DURATION:-60}"
measure_idle "$AMBIENT_DURATION"
AMBIENT=$(cat "$RESULTS_DIR/peak-ambient.txt")
printf 'ambient ceiling (idle %ss peak) = %s\n' "$AMBIENT_DURATION" "$AMBIENT"

# 1 — benign window
drain_floor benign
measure_window benign "$POLLER_BENIGN_DURATION"
P_B=$(cat "$RESULTS_DIR/peak-benign.txt")
printf 'P_b (benign window peak) = %s\n' "$P_B"
extract_peak_nonambient "$RESULTS_DIR/poller-benign.json" \
    "$RESULTS_DIR/peak-benign-nonambient.txt"
P_B_CLEAN=$(cat "$RESULTS_DIR/peak-benign-nonambient.txt")

# 2 — intermittent window
drain_floor intermittent
measure_window intermittent "$POLLER_INTERMITTENT_DURATION"
P_I=$(cat "$RESULTS_DIR/peak-intermittent.txt")
printf 'P_i (intermittent window peak) = %s\n' "$P_I"
extract_peak_nonambient "$RESULTS_DIR/poller-intermittent.json" \
    "$RESULTS_DIR/peak-intermittent-nonambient.txt"
P_I_CLEAN=$(cat "$RESULTS_DIR/peak-intermittent-nonambient.txt")

# 3 — write results (ambient ceiling comes from the idle window above,
# not from the daemon log: the old log-grep measured the bench itself)
printf '{\n  "P_b": %s,\n  "P_i": %s,\n  "P_b_clean": %s,\n  "P_i_clean": %s,\n  "ambient_ceiling_idle": %s,\n  "date": "%s",\n  "comment": "ambient_ceiling_idle = peak V3 price during a 60s idle window (no bench). P_b_eff = max(P_b, ambient_ceiling_idle). P_*_clean = peak over poller samples where dwell_fiber_v3_ambient_storm == 0 (ambient-labeled windows excluded); equals P_* when no storm was seen. Paced intermittent bench (--intermittent-rate, default 350/s); achieved rate printed by bench.py."\n}\n' \
    "$P_B" "$P_I" "$P_B_CLEAN" "$P_I_CLEAN" "$AMBIENT" "$(date -Iseconds)" \
    > "$RESULTS_DIR/results.json"

printf '\n=== results ===\n'
cat "$RESULTS_DIR/results.json"
printf '\n=== done %s ===\n' "$(date -Iseconds)"
