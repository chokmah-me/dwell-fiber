# Project Status

**Last updated:** 2026-06 (v1.6.0)

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

## Frozen

- **V3.0 WIP-based architecture**: drafts in `outputs/` (tracked on `main`),
  preserved at tags `v3.0.0`–`v3.0.2`. There is no active V3 branch.
  Motivated by V2's blind spot for fast intermittent encryption (LockBit 3.0+).
  Not actively developed. The eBPF and Go drafts in `outputs/` are
  unintegrated; tier weights are unvalidated. See `docs/v3-roadmap.md`.
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
   price≈0/killed=0 to detection.
3. Otherwise: stop.
