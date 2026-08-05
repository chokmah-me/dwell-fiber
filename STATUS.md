# Project Status

**Last updated:** 2026-08-04 (v1.7.0 + V3 calibration passes 1–2 + map-stored comm)

## Working

- **V2.x daemon**: eBPF dwell-time tracking, ADMM price updates, throttle (cgroups v2)
  and kill enforcement, Prometheus metrics, web dashboard. Runs on Ubuntu 25.10
  with kernel 5.8+.
- **FD-tracking** (v1.5.0): concurrent file opens in a single process now
  produce distinct dwell events. See `CHANGELOG.md`.
- **Benchmark harness** (v1.6.0): `test/bench.py` runs benign (tar extract),
  sustained-dwell-attack, and fast-intermittent-encryption scenarios; results
  in `BENCHMARKS.md`. `--scenario all` runs all three.
- **Observability** (v1.6.0, corrected in #7): `dwell_fiber_events_total` /
  `dwell_fiber_events_filtered_total` counters distinguish "events seen and
  filtered" from "no events seen". These are now counted **in-kernel before the
  `<100ms` dwell filter** (a per-CPU `stats` array in `dwell_monitor.bpf.c`) —
  as shipped in v1.6.0 they only counted events past that filter, so fast
  intermittent encryption (all sub-100ms) read `0/0`, the exact dead-pipeline
  ambiguity the counters were meant to remove. `dwell_fiber_enforcement_enabled`
  is set at startup (was previously only set after the noise filter).
- **Unit tests** (`daemon/controller_test.go`): 6 tests cover ADMM math
  (average-dwell calculation, price update formula, Lemma 3 non-negativity,
  state return). Run with `make test`. Scheduled to run weekly via GitHub Actions.
- **V3 WIP detection + enforcement** (`--use-v3-wip`): a rate-based Weighted I/O
  Pressure detector running in parallel with V2 (roadmap "dual mode"). Publishes
  `dwell_fiber_v3_*` metrics; on the `intermittent` scenario `v3_wip`/`v3_price`
  rise while V2 `price` stays 0 — the blind spot is *detected*. Signals come from
  syscall tracepoints (TBW from `sys_enter_write`, now filtered in-kernel to
  sub-page/lookup-only to bound overhead; UFM is an opens/s proxy).
  - **Enforcement** (`--v3-enforce`, dry-run by default; `--v3-enable-killing` is
    a separate gate, mirroring V2): high-pressure PIDs are io.max-throttled
    (`pkg/enforcement` `EnforceWIP` / `ThrottleIO`), then killed past
    `V3KillPrice`. Reuses V2's `SafetyChecker` whitelists. Without `--v3-enforce`
    the daemon logs the actions it *would* take.
  - **Price decay**: V3 ADMM price leaks each window (`ControllerV3.Leak`) so a
    transient benign burst bleeds off instead of latching into enforcement range;
    only *sustained* high WIP enforces.
  - Tier budgets (T2 now **150**, `b730d71`) and the
    `V3ThrottlePrice`/`V3KillPrice` thresholds remain documented starting
    points. Both VM calibration passes (2026-08-04, see `BENCHMARKS.md`)
    measured **infeasible separation**: pass 1 (Ubuntu 25.10 VM, budget 300)
    benign peak 542.5 / intermittent 0 / ambient 87.65; pass 2 (WSL Ubuntu
    24.04, budget 150) benign peak 199.95 / intermittent 162.65 / ambient
    floor 162.65. No band exists, so no threshold change was committed.
    Pass 2 confirmed TBW accumulation **works** (1200×1MB probe:
    TBW 298.8–333.4 MB/s, WIP 283–323, price 200.89) and root-caused the
    benign misclassification (`procComm` returning `unknown` → tar defaulted
    to T2).
  - **Map-stored `comm` (post pass 2):** `wip_tracker` now carries
    `char comm[16]` set with `bpf_get_current_comm` on window create;
    `daemon/wip_monitor.go` prefers that name over `/proc/<pid>/comm`. Smoke on
    WSL (2026-08-04): V3 pressure logs show real names (`python3`,
    `localstack`, …) and **no** `(unknown)` for live high-pressure PIDs. Does
    not by itself re-open GATE A/B — re-measure still required after rebuild.

## Frozen

- **V3.0 full WIP**: the remaining V3 work — true unique-inode UFM (needs
  CO-RE/vmlinux.h, replaces the opens/s proxy), ML-based tier classification,
  and budget/threshold calibration against *real ransomware samples* (the
  current values are validated only against the synthetic `bench.py` scenarios).
  The 2026-08-04 calibration passes (pass 1: budget 300, benign 542.5 vs
  intermittent 0; pass 2: budget 150, benign 199.95 vs intermittent 162.65)
  confirmed calibration is not just unstarted but **infeasible against the
  synthetic bench as shipped** (see Working above and `BENCHMARKS.md`). Pass 1
  raised a possible write-path issue; pass 2 **resolved it** — TBW accumulation
  works (probe WIP 283–323 > 150, price 200.89). The attack simply runs below
  budget on the target; pass 2's benign T2 misclassification via `/proc`-only
  `procComm` is **fixed in tree** (map-stored comm — see Working); ambient
  open-storms still exceed the budget. Re-measure GATE A/B after the comm fix.
  cgroups v2 `io.max` throttling + WIP-based killing have landed (see Working).
  Original drafts in `outputs/` (preserved at tags `v3.0.0`–`v3.0.2`) are
  superseded by the integrated daemon above. See `docs/v3-roadmap.md`.
- **Coq proofs**: 29/48 proven (60%). Framework compiles cleanly. The 19
  admitted proofs require Banach fixed-point and temporal-logic machinery
  that is research, not engineering. See `docs/coq_status.md`. No timeline
  for completion.

## Not happening

- A "production-ready" claim. V2 catches sustained-dwell attacks; it does
  not catch fast intermittent encryption. Treat this as a defense-in-depth
  layer, not an EDR replacement.
- A V2.0.0 promise. The benchmarks now demonstrate the V2 blind spot
  empirically (see "What's next" #1), but resuming V3 is a research effort
  gated on external pull (a paper, a deployment, an issue) — not a committed
  release.

## What's next

There is no committed roadmap. Likely follow-ups, in rough priority order:

1. ✅ **Done (v1.6.0).** Added the intermittent-encryption scenario
   (`test/bench.py --scenario intermittent`) and ran it on the Ubuntu target
   against an armed, kill-enabled daemon: 2000 files rewritten, `price` stayed
   at 0, `killed`/`throttled` at 0. The blind spot is confirmed and
   root-caused — short dwells are discarded by two stacked filters before any
   price update: the kernel drops sub-100ms dwells (`dwell_monitor.bpf.c`:
   `if (duration < 100000000)`) and the controller drops sub-1s dwells
   (`daemon/controller.go`: `if dwell < 1*time.Second { return }`). The
   `events`/`filtered` counters (now counted in-kernel pre-filter, #7) show the
   daemon saw and dropped every event — thousands counted, `price` unmoved.
2. **Live next step:** resume V3 (rate-based WIP detection) only on external
   pull. The `intermittent` row is the regression target — V3 must flip it from
   price≈0/killed=0 to detection. Both 2026-08-04 calibration passes (pass 1
   budget 300, pass 2 budget 150) measured **infeasible separation** on the
   synthetic bench; the pass-1 "TBW possibly broken" item is **resolved**
   (pass 2 probe: TBW 298.8–333.4 MB/s, price 200.89 — the write path works).
   Before any enforcement is trusted, in priority order:
   - ✅ **Done (map-stored comm).** BPF `wip_tracker` stores `comm` at window
     create; userspace prefers it over `/proc`. Smoke: real names, zero
     `(unknown)` on V3 pressure lines. Full `v3_measure.sh` re-run still needed
     to confirm tar → T1 and re-score GATE A/B.
   - Recalibrate the T2 budget to the real attack rate on the target (~64
     files/s on the WSL guest, not ~223), or use a faster attack workload: the
     1200×1MB probe (WIP ~290) prices at budget 150 while the 2000×1MB bench
     (WIP ~64) does not.
   - Identify and quiet the ambient enumeration source (~679 opens/s, TBW 0,
     short-lived PIDs, price 162.65 at budget 150) that contaminates
     measurement windows on this host (docker/localstack/containerd candidates).
3. Otherwise: stop.
