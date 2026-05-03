# Project Status

**Last updated:** 2026-05 (v1.5.0)

## Working

- **V2.x daemon**: eBPF dwell-time tracking, ADMM price updates, throttle (cgroups v2)
  and kill enforcement, Prometheus metrics, web dashboard. Runs on Ubuntu 25.10
  with kernel 5.8+.
- **FD-tracking** (v1.5.0): concurrent file opens in a single process now
  produce distinct dwell events. See `CHANGELOG.md`.
- **Benchmark harness** (v1.5.0): `test/bench.py` runs benign (tar extract)
  and sustained-dwell-attack scenarios; results in `BENCHMARKS.md`.

## Frozen

- **V3.0 WIP-based architecture**: drafts on branch `feature/v3-wip-architecture`.
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
- A V2.0.0 promise. v1.5.0 is the last planned release until either the
  benchmarks demonstrate V3 is needed empirically, or there's external
  pull (a paper, a deployment, an issue) for more work.

## What's next

There is no committed roadmap. Likely follow-ups, in rough priority order:

1. Add an intermittent-encryption scenario to the benchmark harness; run
   it against V2 to produce empirical evidence of the failure mode.
2. If (1) shows V2 failing as predicted, resume V3 with the harness as
   regression test.
3. Otherwise: stop.
