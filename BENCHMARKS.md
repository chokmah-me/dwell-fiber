# Dwell-Fiber Benchmarks

Enforcement mode during run: **DRY-RUN (observation only)**. The daemon ran with
`--use-v3-wip` only — no `--v3-enforce`, no killing. Every `dwell_fiber_v3_*`
value below is observation signal; `dwell_fiber_enforcement_enabled` read `0`
throughout.

## Measured runs (2026-08-04, Ubuntu 25.10 VM)

Single daemon instance, default config (`--alpha=0.5 --budget=5.0`). Rows are
the cleanest capture of each scenario: `v3_price` is the bench's own after-scrape
(3 s post-run, leak-decayed); `v3_price_peak` is the parallel
`test/calibrate_v3.py --from-metrics` poller (0.5 s cadence) — the source of
truth, per `docs/v3-calibration.md`.

| scenario | dwell_avg | price | throttled | killed | events | filtered | v3_wip | v3_price | v3_price_peak |
|----------|----------:|------:|----------:|-------:|-------:|---------:|-------:|---------:|--------------:|
| benign (tar extract) | 1.25s | 0.000 | 0 | 0 | 228019 | 227889 | 1385.05 | 439.45 | **542.53** |
| intermittent (2000×1MB) | 1.04s | 0.000 | 0 | 0 | 86846 | 86730 | 156.10 | 0.00 | **0.00** |

- **Benign** (tar extraction, 500 files, ~200KB each): short-dwell opens reach
  a burst UFM of ~1900 files/s, which dominates the T2 WIP
  (`0.3·TBW + 0.7·UFM`), pushing `v3_price` to a poller-measured peak of
  **542.5** (`0.5·(1385 − 300)` — one window of excess above the T2 budget,
  leak math matches exactly).
- **Intermittent** (2000 files, open→write 1MB→close, no hold): the LockBit
  pattern. On this VM the bench runs at ~223 files/s and ~222 MB/s
  (`WIP = 0.3·222 + 0.7·223 ≈ 222`), which never crosses the T2 budget of 300,
  so the bench's own price stays **0.00**. The V2 price also stays 0 (sub-100ms
  dwells filtered in-kernel), so this row still demonstrates the V2 blind spot —
  but the V3 detector does **not** fire on it as shipped.

### Ambient open-storm (contaminates every window)

A recurring burst of short-lived processes (new PID each time, `(unknown)` =
exited within the window) opened **451–679 files/s with TBW=0** every ~15–90 s
all session — most often exactly 679/s (a fixed ~679-entry directory tree being
enumerated, consistent with a Windows/WSL-interop sync scan on this
Windows-hosted VM). `0.7·679 = 475 > 300` budget, so it prices out at **87.65**
on an otherwise idle VM. The session-1 "intermittent peak" of 87.65 was this
noise, not the benchmark.

## Threshold calibration — GATE A/B/C (2026-08-04)

Per `docs/v3-calibration.md`, this pass was the documented v1.7.0 prerequisite
(re-tune on the VM before trusting live V3 enforcement). It was never done
before; it is now measured.

| Gate | Requirement | Result |
|------|-------------|--------|
| **GATE A** | benign peak `v3_price` < `V3ThrottlePrice` | **FAILS** — benign peaks at 477–542, above both current defaults (50) and any plausible throttle that also satisfies GATE B |
| **GATE B** | intermittent peak ≥ `V3ThrottlePrice` | **FAILS** — the intermittent bench price is 0; it never crosses the T2 budget |
| **GATE C** | `V3KillPrice` > `V3ThrottlePrice` | moot — no band exists to place |

Measured peaks: **P_b = 542.53** (benign, poller), **P_i = 0.00** (intermittent).
`P_i ≤ P_b` → separation is **infeasible**; the harness rule is explicit: do not
invent a band. **No `V3ThrottlePrice` / `V3KillPrice` change was committed** —
50/150 remain starting points.

With the current defaults the false-positive/false-negative picture is the
inverse of the v1.7.0 assumption: benign tar extraction (542) and the ambient
open-storm (87.65) both cross `V3ThrottlePrice=50` (benign even crosses
`V3KillPrice=150`), while the attack pattern never reaches 50.

### Root causes (all plan-listed investigation items)

1. **T2 budget/weights vs real attack rate.** The harness's synthetic
   `intermittent_attack` profile assumes UFM ≈ 800/s (WIP 596); the real bench
   on this VM sustains ~223 files/s + ~222 MB/s → WIP ≈ 222 < budget 300. The
   attack cannot price out at the configured budget.
2. **TBW not observed.** `sys_enter_write` accumulation (`tbw_accum`) read 0 for
   every budget-crossing write workload attempted (500×1MB probe, both
   intermittent runs); the only nonzero TBW readings appeared on benign phases.
   Tracepoint `count` offset was **verified correct** at 32
   (`/sys/kernel/tracing/events/syscalls/sys_enter_write/format`), so this is
   not a fixed-offset bug — the reliability of the write path is unresolved and
   needs an isolated budget-crossing write experiment.
3. **Ambient noise exceeds budget.** The recurring enumeration (UFM ~679/s,
   TBW 0) prices at 87.65 on an idle VM, so no threshold near the attack band
   can avoid firing on ambient.
4. **Tier/budget are MVP placeholders** (documented in `controller_v3.go` and
   `STATUS.md` Frozen) — this pass measured against them as shipped.

### Context

- Date: 2026-08-04. Target: Ubuntu 25.10 (Windows-hosted VM, hostname
  DESKTOP-7HDI2D1), Go 1.24.9, clang 18.1.3, libbpf 1.3.0. Disk: ~222 MB/s
  sustained write (2 GB intermittent run ≈ 9 s), fast extract bursts.
- Instrumentation: `sudo ./bin/dwell-fiber-daemon --use-v3-wip` (observation
  only) + parallel `calibrate_v3.py --from-metrics` pollers + bench.py
  before/after scrapes + daemon `📈 [V3]` per-window logs.
- Reproducible decision path in `docs/v3-calibration.md` ("Measured run record",
  2026-08-04).

## Follow-ups (not done in this pass — no BPF/controller/WIP code changes)

1. Recalibrate the T2 budget/weights (or speed the bench) so the real
   intermittent rate can cross — then re-run GATE A/B.
2. Isolate the TBW write-accumulation path with a budget-crossing pure-write
   workload (no opens), then decide if a BPF fix is needed.
3. Identify and quiet the ambient enumeration source (WSL interop scan) before
   trusting any near-budget threshold.
