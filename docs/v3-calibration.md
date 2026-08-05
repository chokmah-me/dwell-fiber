# V3 Threshold Calibration

**Purpose:** VM re-tune of `V3ThrottlePrice` and `V3KillPrice` so enforcement
gates match real benign vs intermittent WIP price peaks. Production defaults in
`pkg/enforcement/config.go` are **starting points only** (50 / 150); this doc and
`test/calibrate_v3.py` help you pick numbers without guessing.

See also: `STATUS.md` (Working vs Frozen V3 items), `CHANGELOG.md` threshold note,
and `docs/v3-roadmap.md` calibration status.

---

## Prerequisites

1. Ubuntu host (or VM) with the dwell-fiber daemon built.
2. Run observation first so you can scrape prices without side effects:
   ```bash
   sudo ./bin/dwell-fiber-daemon --use-v3-wip
   ```
3. After thresholds look right, re-run with enforcement (still dry-run until you
   enable killing if desired):
   ```bash
   sudo ./bin/dwell-fiber-daemon --use-v3-wip --v3-enforce
   ```
4. Metrics on `http://localhost:9090/metrics` (especially `dwell_fiber_v3_price`).

No root is required for the **offline** harness (`--from-peaks`, `--simulate`).

---

## Procedure

1. **Bench benign** (tar extract) and record peak `dwell_fiber_v3_price`:
   ```bash
   python3 test/bench.py --scenario benign
   ```
   Note the after/peak `v3_price` (or poll with
   `python3 test/calibrate_v3.py --from-metrics --duration-s 30` during the run).

2. **Bench intermittent** (fast open→write→close) and record peak:
   ```bash
   python3 test/bench.py --scenario intermittent
   ```

3. **Calibrate offline** from the two peaks:
   ```bash
   python3 test/calibrate_v3.py --from-peaks \
     --benign-peak P_b --intermittent-peak P_i
   ```
   Optional: `--margin-throttle 0.15` (default), `--kill-ratio 2.0` (default).

4. **Apply config by hand** — set `V3ThrottlePrice` / `V3KillPrice` in the
   enforcement config (or whatever deployment path you use). This harness does
   **not** edit Go sources.

5. **Re-bench** under `--use-v3-wip` / `--v3-enforce` and confirm the gates below.

### Offline simulation (no daemon)

```bash
python3 test/calibrate_v3.py --simulate --profile benign_tar
python3 test/calibrate_v3.py --simulate --profile intermittent_attack
python3 test/calibrate_v3.py --simulate --samples-jsonl path/to/samples.jsonl --tier t2
```

Simulation mirrors `daemon/controller_v3.go`:

- `price = max(0, leak(price) + α*(WIP − budget))`
- **leak**: multiply by `0.9` (`defaultLeak`), then snap to `0` if result `< 0.5`
- **T2** (default): WIP = `0.3·TBW + 0.7·UFM`, budget `150`
  (since `b730d71`; was 300 before the pass-2 experiment)
- Tier budgets T1=3000 / T1.5=1500 / T2=150 remain **MVP placeholders** — not
  fully calibrated against real ransomware samples (see STATUS.md Frozen).

---

## Regression gates (GATE A / B / C)

| Gate | Requirement |
|------|-------------|
| **GATE A** | Benign peak `v3_price` stays **below** `V3ThrottlePrice` |
| **GATE B** | Intermittent peak **reaches or exceeds** `V3ThrottlePrice` |
| **GATE C** | `V3KillPrice` **>** `V3ThrottlePrice` with margin (default kill = throttle × 2) |

Formula used by the harness when `P_i > P_b`:

```
throttle = P_b + M * (P_i - P_b)     # M default 0.15
kill     = throttle * R              # R default 2.0
```

If `P_i ≤ P_b`, separation is infeasible: all gates report false and notes
explain that the attack did not out-price benign (re-bench or re-check leak /
tier / load).

---

## Notes

- **Words for search / agents:** `V3ThrottlePrice`, `V3KillPrice`, intermittent,
  benign, leak, GATE — these are the knobs and gates this doc owns.
- Tier budgets and UFM (opens/s proxy vs true unique-inode) are still MVP; do not
  invent CO-RE/UFM claims. True inode UFM remains Frozen in STATUS.md.
- **Tier names:** V3 resolves process names from the BPF map first (`comm` on
  `wip_state`), then `/proc/<pid>/comm`. Name-based tiers (`tar` → T1) need a
  live re-measure after the map-comm fix; do not assume pass-2 peaks still hold.
- Unit check: `python test/test_calibrate_v3.py` (stdlib only, exit 0).

---

## Measured run record — 2026-08-04 (Ubuntu 25.10 VM)

**Outcome: infeasible separation — no threshold change committed.** This is the
measured record the v1.7.0 release note ("re-tune on the VM before trusting live
V3 enforcement") required. Full report: `BENCHMARKS.md`.

### Environment

- Ubuntu 25.10 (Windows-hosted VM, `DESKTOP-7HDI2D1`); Go 1.24.9, clang 18.1.3,
  libbpf 1.3.0. Sustained write ~222 MB/s (2 GB intermittent run ≈ 9 s).
- Daemon: `sudo ./bin/dwell-fiber-daemon --use-v3-wip` (observation only,
  `dwell_fiber_enforcement_enabled 0` throughout). Peaks from parallel
  `calibrate_v3.py --from-metrics` pollers (0.5 s cadence).

### Measured peaks and gates

| Scenario | Peak WIP | Poller peak `v3_price` | Gate |
|----------|---------:|-----------------------:|------|
| Benign (tar extract, 500×~200KB) | 1385 (`0.3·176.5 + 0.7·1903`) | **542.53** | A **FAILS** vs 50 (and > 150) |
| Intermittent (2000×1MB, ~223 files/s) | 222 (`0.3·222 + 0.7·223`) | **0.00** | B **FAILS** (below T2 budget 300) |
| Ambient enumeration (~1/min) | 475 (`0.7·679`, TBW 0) | **87.65** | — (false-positive vs 50 on idle VM) |

Leak math check (matches single-window excess): benign `0.5·(1385−300) = 542.5`;
ambient `0.5·(475−300) = 87.5`; intermittent `0.5·(222−300) < 0 → 0`.

`P_b = 542.53`, `P_i = 0.00` → `P_i ≤ P_b`, the harness's documented infeasible
case: all gates false, no band to place, thresholds stay starting points.

### Investigation notes (plan-listed checks)

1. **Write filter / TBW.** `count < 4096` skip is not the issue (1 MB writes
   pass). Tracepoint layout verified: `sys_enter_write` `count` is at
   `offset:32` — the BPF read (`ctx + 32`) is correct, so no fixed offset bug.
   Yet `tbw_accum` read 0 on every budget-crossing write workload attempted
   (500×1MB probe, intermittent runs); the only nonzero TBW readings occurred on
   benign phases. Reliability of the write path is unresolved and needs an
   isolated budget-crossing pure-write experiment.
2. **Tier classification.** Attack writer is `python3` → T2 (name-based),
   confirmed in daemon logs and tests.
3. **Leak/budget.** T2 budget 300 exceeds the real bench WIP (~222); benign's
   UFM burst (1900/s) crosses via the UFM-heavy weights. Both peaks equal one
   window of excess — bursts are shorter than the leak's accumulation horizon.
4. **Ambient noise.** Recurring short-lived `(unknown)` PIDs, UFM 451–679/s,
   TBW 0, ~15–90 s cadence, most often exactly 679/s — consistent with a
   periodic ~679-entry directory enumeration (Windows/WSL-interop sync scan).
   It exceeds the T2 budget on an idle VM and appears in every measurement
   window.

### Decision

No `V3ThrottlePrice` / `V3KillPrice` change. Enabling `--v3-enforce` on this
build would throttle benign tar extracts and the ambient noise while missing the
intermittent pattern. Follow-ups (no code changes in this pass): recalibrate T2
budget/weights to the real rate; isolate TBW; identify the ambient source.

---

## Measured run record — 2026-08-04 (second pass, WSL Ubuntu 24.04, T2 budget 150)

**Outcome: infeasible separation again — no threshold change committed.** This
pass lowered the T2 budget to 150 (`b730d71`), pre-generated the benign tar (the
measured window is a pure `tar -xf`, no in-window build), and re-ran the benign
+ intermittent windows. Full report: `BENCHMARKS.md`.

### Environment

- WSL Ubuntu 24.04 guest (`Ubuntu-24.04`, `DESKTOP-7HDI2D1`, user `dyb`); Go
  toolchain in-guest. Root disk `/dev/sdd` (~941 GB free). Probe workload
  averaged ~98 MB/s with per-window bursts up to 333 MB/s.
- Daemon: `sudo ./bin/dwell-fiber-daemon --use-v3-wip` (observation only,
  `dwell_fiber_enforcement_enabled 0` throughout). Peaks from parallel
  `calibrate_v3.py --from-metrics` pollers (0.5 s cadence) + bench before/after
  scrapes + daemon `📈 [V3]` per-window logs.

### Measured peaks and gates

| Scenario | Peak WIP | Poller peak `v3_price` | Gate |
|----------|---------:|-----------------------:|------|
| Benign (tar extract, 500×~200KB) | 206–532 (`0.3·28…71 + 0.7·282…730`) | **199.95** | A **FAILS** vs 50 (and > 150) |
| Intermittent (2000×1MB, ~64 files/s) | ~167 (`UFM 238`, TBW 0) | **162.65** (ambient; bench's own ~8) | B **FAILS** as a signal |
| Ambient enumeration (~30 s cadence) | 475 (`0.7·679`, TBW 0) | **162.65** | — (floor at budget 150) |

`P_b = 199.95`, `P_i = 162.65` → `P_i ≤ P_b`, the harness's documented
infeasible case: all gates false, no band to place, thresholds stay starting
points.

### Investigation notes (pass-2 answers to pass-1's open items)

1. **TBW accumulation works (pass-1 item resolved).** The 1200×1MB probe
   (`test/tbw_probe.py`) read `TBW = 298.8–333.4 MB/s` (daemon log, PID 20539),
   WIP 283–323 > budget 150, price → **200.89**; an earlier probe read
   TBW 195 MB/s (PID 23793). The `sys_enter_write` write-accumulation path is
   live — the pass-1 "TBW not observed" question was a workload/timing artifact
   of the 25.10 VM bench, not a BPF bug. No BPF write-path fix is needed.
2. **Attack rate below budget on this target.** The intermittent bench ran
   2000×1MB in **31.0 s** ≈ 64 files/s (the 25.10 VM did ~223/s). WIP ≈ 64 <
   budget 150 → the bench's own price peaked at ~8 (log: `UFM=238/s WIP=167
   price=8.3`). The 162.65 window peak is ambient. The budget now sits between
   the probe (WIP ~290, prices) and the bench (WIP ~64, does not).
3. **Benign misclassification via `procComm` (pass-2 root cause; fixed in
   tree).** The tar extract was classified **T2**, not T1: `/proc`-only
   `procComm` returned `unknown` under WSL PID skew (probe self-report 278314
   vs BPF-observed 20539), and `ClassifyTier("unknown")` defaults to T2. The
   tar's T2 WIP 206–532 priced to 199.95 in the benign window — above the
   attack. **Post-pass fix:** store `comm` in `wip_tracker` at window create
   (`bpf_get_current_comm`); userspace `resolveComm` prefers the map. Smoke
   after rebuild shows real names on V3 pressure lines (no `(unknown)`).
   Re-run this calibration procedure (`test/v3_measure.sh`) before treating
   GATE A as recovered.
4. **Ambient floor at budget 150: 162.65.** `0.5×(0.7·679−150)`; bursts every
   ~30 s, price never drains to literal 0.

### Decision

No `V3ThrottlePrice` / `V3KillPrice` change (second infeasibility; same
conclusion as pass 1, different root causes). Follow-ups after this pass: the
`procComm` map-comm fix is in tree (see CHANGELOG Unreleased); still re-measure
GATE A/B, re-calibrate T2 budget to the real attack rate (~64 files/s here) or
use a faster attack workload, and identify the ambient enumeration source.
