#!/usr/bin/env bash
# wsl_throttle_test.sh -- item 6: throttle-only armed V3 enforcement test (WSL).
#
# The real gate for V3 enforcement: arm --v3-enforce (io.max throttle, and
# deliberately NO --v3-enable-killing) on the BPF-capable WSL Ubuntu 24.04
# guest during the intermittent bench, and confirm the attack gets throttled
# while the benign tar is untouched.
#
# Run INSIDE the dwell-fiber repo on the WSL guest:
#     cd ~/dwell-fiber && bash test/wsl_throttle_test.sh
#
# Does: git pull, make daemon, prepare benign.tar, (re)start the daemon with
#   --use-v3-wip --v3-enforce, benign window, drain, armed attack (relaunch
#   loop until the throttle engages, then a 20s throttled-state evidence
#   window), hard-gate assertions, summary.
#
# Why a relaunch loop instead of N bench runs: the 1 MB/s io.max cap grinds
# the 350 MB/s bench to ~1 file/s once engaged, so a fixed number of full
# runs would take hours. The loop sustains pressure only until the throttle
# is confirmed, then stops the attack.
#
# Hard gates (any FAIL => exit 1):
#   1. attack throttled: dwell_fiber_v3_throttled_count rises during the
#      intermittent window (daemon log shows "[io] Throttling" for it)
#   2. no killing: dwell_fiber_v3_killed_count unchanged. The kill price WILL
#      be crossed (P_i >> 204.4); the daemon must log "[DRY-RUN] Would kill",
#      never kill. This is the proof the kill gate stayed disarmed.
#   3. kernel-level cap present: dwell-fiber-v3.slice/io.max carries
#      wbps=1048576 (1 MB/s)
#   4. benign tar untouched: tar exits 0 and throttled_count is unchanged
#      across the benign window
# Supporting evidence (reported, not gating -- host-dependent):
#   - time-to-throttle: seconds from attack start to first throttle
#   - throttled 20s window: slice io.stat write rate (cap => ~1 MB/s) and
#     files written (= effective attack rate under the throttle)
#
# Safety notes (verified against pkg/enforcement on 2026-09-28):
#   - Killer.KillNow checks KillEnabled FIRST: with --v3-enable-killing off it
#     only logs "[DRY-RUN] Would kill". Throttle-only arming cannot kill.
#   - The throttle is sticky: once a PID crosses 102.2 it is moved into
#     dwell-fiber-v3.slice and nothing removes it (the kill-price early return
#     in EnforceWIP only skips re-throttling, it does not un-throttle).
#   - SafetyChecker refuses protected PIDs/cmds and the daemon itself.
#
# Needs: sudo (eBPF + cgroups), curl, python3. Takes ~4-7 minutes.
# FAILS CLOSED if the daemon is in simulation mode or not armed throttle-only.
set -u
export LC_ALL=C

REPO="${REPO:-$PWD}"
RESULTS_DIR="${RESULTS_DIR:-/tmp/v3-throttle-test}"
DAEMON_LOG="${DAEMON_LOG:-/tmp/daemon-v3b-throttle.log}"
METRICS_URL="http://localhost:9090/metrics"
THROTTLE_PRICE=102.2
KILL_PRICE=204.4
EXPECTED_WBPS=1048576
DRAIN_THRESHOLD=50
DRAIN_TIMEOUT_S=120
INTERMITTENT_RATE="${INTERMITTENT_RATE:-350}"
BENCH_WORKDIR="${BENCH_WORKDIR:-/tmp/dwell-fiber-bench}"
CGROUP_SLICE="/sys/fs/cgroup/dwell-fiber-v3.slice"

mkdir -p "$RESULTS_DIR"

die() { printf 'FATAL: %s\n' "$*" >&2; exit 1; }

metric() { # metric <name> -> value or empty
    curl -s --max-time 2 "$METRICS_URL" 2>/dev/null \
        | grep "^$1" | tr -s ' ' | cut -d' ' -f2 | head -1
}

# sample_max_price <duration_s> <outfile>: poll dwell_fiber_v3_price every 2s.
# Writes the running max to <outfile> on every poll, so killing the sampler
# early still leaves the max observed so far.
sample_max_price() {
    local duration="$1" outfile="$2" t0 now max=0 v
    t0=$(date +%s)
    printf '0\n' > "$outfile"
    while true; do
        now=$(date +%s)
        [ $((now - t0)) -ge "$duration" ] && break
        v=$(metric dwell_fiber_v3_price); v="${v:-0}"
        if python3 -c "import sys; sys.exit(0 if float(sys.argv[1]) > float(sys.argv[2]) else 1)" \
                "$v" "$max" 2>/dev/null; then
            max="$v"
            printf '%s\n' "$max" > "$outfile"
        fi
        sleep 2
    done
}

# stop_sampler <pid>: stop a background sample_max_price without waiting out
# its full duration (it already wrote the running max).
stop_sampler() {
    kill "$1" 2>/dev/null || true
    wait "$1" 2>/dev/null || true
}

drain() {
    local t0 elapsed p
    t0=$(date +%s)
    while true; do
        elapsed=$(( $(date +%s) - t0 ))
        if [ "$elapsed" -ge "$DRAIN_TIMEOUT_S" ]; then
            printf 'drain: timeout at %ss -- proceeding\n' "$elapsed"; break
        fi
        p=$(metric dwell_fiber_v3_price); p="${p:-0}"
        if python3 -c "import sys; sys.exit(0 if float(sys.argv[1]) < float(sys.argv[2]) else 1)" \
                "$p" "$DRAIN_THRESHOLD" 2>/dev/null; then
            printf 'drain: price %s < %s at %ss\n' "$p" "$DRAIN_THRESHOLD" "$elapsed"; break
        fi
        sleep 5
    done
}

io_wbytes() { # total wbytes attributed to the throttle slice, or empty
    awk '{for(i=1;i<=NF;i++){if($i ~ /^wbytes=/){split($i,a,"="); s+=a[2]}}} END{printf "%d", s+0}' \
        "$CGROUP_SLICE/io.stat" 2>/dev/null
}

# ============ pre-flight ============
[ -f "$REPO/test/v3_measure.sh" ] || die "run from the dwell-fiber repo root"
cd "$REPO"

printf '=== 1/7 git pull ===\n'
git pull --ff-only 2>/dev/null || printf 'WARN: git pull failed; continuing with working tree\n'

printf '=== 2/7 make daemon ===\n'
make daemon || die "make daemon failed"

printf '=== 3/7 prepare benign.tar ===\n'
python3 test/bench.py --prepare-tar --workdir "$BENCH_WORKDIR" || die "bench --prepare-tar failed"

# The throttle applies io.max to the block device backing the daemon's
# target PID cwd (we run everything from $REPO). If the bench workdir lives
# on a different device, the cap would land on the wrong device: warn loudly.
repo_dev=$(stat -c '%d' "$REPO" 2>/dev/null || echo "?")
work_dev=$(stat -c '%d' "$BENCH_WORKDIR" 2>/dev/null || echo "?")
printf 'device check: repo=%s workdir=%s\n' "$repo_dev" "$work_dev"
[ "$repo_dev" = "$work_dev" ] || printf 'WARN: repo and bench workdir are on different devices -- the io.max cap follows the PID cwd device; verify the throttle bit with io.stat below\n'

printf '=== 4/7 (re)start daemon ARMED: --use-v3-wip --v3-enforce (NO killing) ===\n'
sudo pkill -f dwell-fiber-daemon 2>/dev/null || true
sleep 2
if curl -s --max-time 2 "$METRICS_URL" | grep -q '^dwell_fiber_v3_price'; then
    die "port 9090 still serving after pkill -- is another dwell-fiber-daemon running? (sudo ss -ltnp | grep 9090)"
fi
# shellcheck disable=SC2024
sudo ./bin/dwell-fiber-daemon --use-v3-wip --v3-enforce \
    --v3-throttle-price "$THROTTLE_PRICE" --v3-kill-price "$KILL_PRICE" \
    > "$DAEMON_LOG" 2>&1 &
echo $! > /tmp/daemon-v3b-throttle.pid
for i in $(seq 1 30); do
    curl -s --max-time 2 "$METRICS_URL" | grep -q '^dwell_fiber_price' && break
    if [ "$i" = 30 ]; then die "daemon metrics not reachable after 30s; see $DAEMON_LOG"; fi
    sleep 1
done
# Fail closed on simulation mode: armed enforcement is meaningless there.
if ! curl -s --max-time 2 "$METRICS_URL" | grep -q '^dwell_fiber_v3_price'; then
    die "dwell_fiber_v3_price not exported -- daemon is in SIMULATION mode (BPF failed to load). Armed test is meaningless; check $DAEMON_LOG"
fi
# Fail closed on arming: must say throttle-only, must NOT say + KILL.
grep -q 'V3 enforcement: io.max throttle (' "$DAEMON_LOG" \
    || die "daemon did not arm io.max throttle -- see $DAEMON_LOG"
if grep -q 'V3 enforcement: io.max throttle + KILL' "$DAEMON_LOG"; then
    die "KILL is armed -- refusing to run the throttle-only test. Restart without --v3-enable-killing."
fi
printf 'armed OK: throttle-only (kill disarmed). throttled=%s killed=%s\n' \
    "$(metric dwell_fiber_v3_throttled_count)" "$(metric dwell_fiber_v3_killed_count)"

# ============ phase A: benign (must stay untouched) ============
printf '=== 5/7 phase A: benign tar (armed daemon watching) ===\n'
thr_before_a=$(metric dwell_fiber_v3_throttled_count); thr_before_a="${thr_before_a:-0}"
sample_max_price 60 "$RESULTS_DIR/peak-benign.txt" &
sampler_pid=$!
set +e
python3 test/bench.py --scenario benign --workdir "$BENCH_WORKDIR" \
    > "$RESULTS_DIR/bench-benign.log" 2>&1
benign_rc=$?
set -e
stop_sampler "$sampler_pid"
thr_after_a=$(metric dwell_fiber_v3_throttled_count); thr_after_a="${thr_after_a:-0}"
peak_a=$(cat "$RESULTS_DIR/peak-benign.txt")
printf 'benign: tar exit=%s peak_price=%s throttled %s -> %s\n' \
    "$benign_rc" "$peak_a" "$thr_before_a" "$thr_after_a"

printf '=== drain before attack window ===\n'
drain

# ============ phase B: intermittent attack (armed) ============
printf '=== 6/7 phase B: intermittent attack (ARMED) ===\n'
printf 'NOTE: the 1 MB/s io.max cap grinds the 350 MB/s bench to ~1 file/s once\n'
printf '      the throttle engages -- that collapse IS the gate working. So the\n'
printf '      attack runs in a relaunch loop only until the throttle is\n'
printf '      confirmed (a single 2000-file run may finish before the price\n'
printf '      crosses 102.2); then we measure the throttled state and stop it.\n'
thr_before_b=$(metric dwell_fiber_v3_throttled_count); thr_before_b="${thr_before_b:-0}"
kill_before_b=$(metric dwell_fiber_v3_killed_count); kill_before_b="${kill_before_b:-0}"
t0=$(date +%s)
sample_max_price 900 "$RESULTS_DIR/peak-intermittent.txt" &
sampler_pid=$!
# Watch the throttle slice's membership during the attack: a bench PID is
# moved in mid-run and exits at iteration end, so sampling between runs would
# miss it. Any PID ever seen here during phase B = a live throttle.
: > "$RESULTS_DIR/slice_pids.txt"
( while true; do
    cat "$CGROUP_SLICE/cgroup.procs" 2>/dev/null >> "$RESULTS_DIR/slice_pids.txt"
    sleep 1
done ) &
watcher_pid=$!
: > "$RESULTS_DIR/bench-intermittent.log"
# Attack loop: re-launch the bench whenever it exits, until the throttle
# engages. Sustained pressure guarantees the 102.2 crossing is observed even
# if one run finishes first.
( while true; do
    python3 test/bench.py --scenario intermittent \
        --intermittent-rate "$INTERMITTENT_RATE" --workdir "$BENCH_WORKDIR" \
        >>"$RESULTS_DIR/bench-intermittent.log" 2>&1
    sleep 1
done ) &
attack_pid=$!

# Wait for the throttle to engage (up to 180s, then fail closed).
engaged=0; t_eng=0
for i in $(seq 1 90); do
    thr_now=$(metric dwell_fiber_v3_throttled_count); thr_now="${thr_now:-0}"
    if [ "$thr_now" -gt "$thr_before_b" ]; then
        engaged=1; t_eng=$(( $(date +%s) - t0 )); break
    fi
    sleep 2
done
if [ "$engaged" = 1 ]; then
    printf 'throttle ENGAGED (time-to-throttle %ss)\n' "$t_eng"
else
    printf 'throttle did NOT engage within 180s -- gates below will FAIL\n'
fi

# Throttled-state evidence window (20s): the slice may only ever contain
# throttled PIDs, so its kernel write rate must sit at/below the 1 MB/s cap;
# count files written in the window as the effective attack rate under throttle.
wrate="?"
nfiles_win="?"
if [ "$engaged" = 1 ]; then
    wa=$(io_wbytes); wa="${wa:-0}"
    sleep 20
    wb=$(io_wbytes); wb="${wb:-0}"
    wrate=$(python3 -c "print(f'{($wb - $wa) / 20 / 1e6:.2f}')" 2>/dev/null || echo "?")
    nfiles_win=$(find "$BENCH_WORKDIR/intermittent_out" -name 'victim_*.dat' \
        -newermt '-25 seconds' 2>/dev/null | wc -l)
    printf 'throttled window: slice write rate=%s MB/s (cap 1.00), files written in 20s=%s\n' \
        "$wrate" "$nfiles_win"
    printf '%s\n' "$wrate" > "$RESULTS_DIR/throttled-wrate.txt"
    printf '%s\n' "$nfiles_win" > "$RESULTS_DIR/throttled-files.txt"
fi

# Stop the attack: kill the relaunch loop, then any in-flight bench.
kill "$attack_pid" 2>/dev/null || true
pkill -f "bench.py --scenario intermittent" 2>/dev/null || true
wait "$attack_pid" 2>/dev/null || true
sleep 2
kill "$watcher_pid" 2>/dev/null || true
wait "$watcher_pid" 2>/dev/null || true
stop_sampler "$sampler_pid"
thr_after_b=$(metric dwell_fiber_v3_throttled_count); thr_after_b="${thr_after_b:-0}"
kill_after_b=$(metric dwell_fiber_v3_killed_count); kill_after_b="${kill_after_b:-0}"
peak_b=$(cat "$RESULTS_DIR/peak-intermittent.txt")
printf 'attack: peak_price=%s throttled %s -> %s | killed %s -> %s\n' \
    "$peak_b" "$thr_before_b" "$thr_after_b" "$kill_before_b" "$kill_after_b"

# ============ 7/7 assertions ============
printf '\n=== 7/7 gate assertions ===\n'
fails=0
gate() { # gate <name> <0|1> <detail>
    if [ "$2" = 1 ]; then printf '  [PASS] %s -- %s\n' "$1" "$3";
    else printf '  [FAIL] %s -- %s\n' "$1" "$3"; fails=$((fails+1)); fi
}

# 1. attack throttled
thr_delta_b=$((thr_after_b - thr_before_b))
gate "attack throttled (throttled_count +$thr_delta_b in attack window)" \
    "$([ "$thr_delta_b" -ge 1 ] && echo 1 || echo 0)" \
    "throttled_count $thr_before_b -> $thr_after_b"
n_thr_log=$(grep -c '\[io\] Throttling PID=' "$DAEMON_LOG" 2>/dev/null || true)
n_thr_log="${n_thr_log:-0}"
gate "daemon logged [io] Throttling ($n_thr_log lines)" \
    "$([ "$n_thr_log" -ge 1 ] && echo 1 || echo 0)" \
    "see '$DAEMON_LOG'"

# 2. no killing
kill_delta_b=$((kill_after_b - kill_before_b))
gate "no killing (killed_count +$kill_delta_b)" \
    "$([ "$kill_delta_b" -eq 0 ] && echo 1 || echo 0)" \
    "killed_count $kill_before_b -> $kill_after_b"
n_wouldkill=$(grep -c 'DRY-RUN.*Would kill' "$DAEMON_LOG" 2>/dev/null || true)
n_wouldkill="${n_wouldkill:-0}"
printf '  [INFO] kill price crossed but disarmed: %s "[DRY-RUN] Would kill" lines\n' "$n_wouldkill"

# 3. kernel-level cap present
cap_ok=0
if [ -f "$CGROUP_SLICE/io.max" ] && grep -q "wbps=$EXPECTED_WBPS" "$CGROUP_SLICE/io.max" 2>/dev/null; then
    cap_ok=1
fi
gate "io.max cap wbps=$EXPECTED_WBPS on $CGROUP_SLICE" "$cap_ok" \
    "$(tr '\n' ' ' < "$CGROUP_SLICE/io.max" 2>/dev/null || echo 'io.max missing')"
procs_seen=0
pids_in_slice=""
if [ -s "$RESULTS_DIR/slice_pids.txt" ]; then
    procs_seen=1
    pids_in_slice=$(sort -u "$RESULTS_DIR/slice_pids.txt" | tr '\n' ' ')
fi
# Fallback: the throttler logs "[io] Throttling PID=<pid>" at the moment it
# moves the PID into the slice. If polling missed it (PID exited between
# 1s samples), the log proves the PID was throttled.
if [ "$procs_seen" -eq 0 ]; then
    logged_pids=$(grep -o '\[io\] Throttling PID=[0-9]*' "$DAEMON_LOG" 2>/dev/null | grep -o '[0-9]*' | sort -u | tr '\n' ' ')
    if [ -n "$logged_pids" ]; then
        procs_seen=1
        pids_in_slice="$logged_pids (from daemon log)"
    fi
fi
gate "attack PID observed in slice cgroup.procs during run" "$procs_seen" \
    "pids seen in slice: ${pids_in_slice:-none}"

# 4. benign untouched
gate "benign tar exit 0" "$([ "$benign_rc" -eq 0 ] && echo 1 || echo 0)" \
    "exit=$benign_rc (log: $RESULTS_DIR/bench-benign.log)"
thr_delta_a=$((thr_after_a - thr_before_a))
gate "benign caused no throttling (+$thr_delta_a)" \
    "$([ "$thr_delta_a" -eq 0 ] && echo 1 || echo 0)" \
    "throttled_count $thr_before_a -> $thr_after_a, peak_price=$peak_a"

printf '\n  supporting evidence (not gating):\n'
printf '  - attack peak price: %s (throttle=%.1f kill=%.1f)\n' "$peak_b" "$THROTTLE_PRICE" "$KILL_PRICE"
if [ "$engaged" = 1 ]; then
    frate=$(python3 -c "print(f'{$nfiles_win/20:.1f}')" 2>/dev/null || echo "?")
    printf '  - time-to-throttle: %ss\n' "$t_eng"
    printf '  - throttled 20s window: slice write rate %s MB/s (cap 1.00), %s files written (~%s/s vs %s/s unthrottled)\n' \
        "$wrate" "$nfiles_win" "$frate" "$INTERMITTENT_RATE"
else
    printf '  - throttle never engaged; no throttled-state evidence collected\n'
fi

printf '\n=== %s (%s/%s hard gates failed) ===\n' \
    "$([ "$fails" -eq 0 ] && echo 'THROTTLE TEST: PASS' || echo 'THROTTLE TEST: FAIL')" \
    "$fails" "7"
printf 'results: %s | daemon log: %s\n' "$RESULTS_DIR" "$DAEMON_LOG"
printf '\nNOTE: daemon is still ARMED. Restore observation mode when done:\n'
printf '  sudo pkill -f dwell-fiber-daemon\n'
printf '  sudo ./bin/dwell-fiber-daemon --use-v3-wip > /tmp/daemon-v3b.log 2>&1 &\n'

[ "$fails" -eq 0 ] || exit 1
