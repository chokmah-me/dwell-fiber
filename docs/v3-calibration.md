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
- **T2** (default): WIP = `0.3·TBW + 0.7·UFM`, budget `300`
- Tier budgets T1=3000 / T1.5=1500 / T2=300 remain **MVP placeholders** — not
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
