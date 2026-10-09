# Dwell-Fiber Benchmarks

Enforcement mode during run: **DRY-RUN (observation only)**

Two runs of the same scenario against a single daemon instance with default
config (`--alpha=0.5 --budget=5.0`): the first without V3 (the V2.x blind
spot), the second with `--use-v3-wip` observation on the WSL guest
(2026-10-08, daemon log `/tmp/daemon-v3b-throttle.log`).

| scenario | elapsed | dwell_avg | price | throttled | killed | events | filtered | v3_wip | v3_price |
|----------|--------:|----------:|------:|----------:|-------:|-------:|---------:|-------:|---------:|
| intermittent (V2 only, no `--use-v3-wip`) | 7.3s | 0.00s | 0.000 | +0 | +0 | +2741 | +2741 | 0 | 0.000 |
| intermittent (V3 observation, WSL 2026-10-08) | 6.0s | 0.00s | 0.000 | +0 | +0 | +2807 | +2807 | 254 | 369.549 |

## What this shows

**Intermittent** (2000 files, open->write 1MB->close, no hold): the
LockBit 3.0+ fast-intermittent-encryption pattern. Each file session is
sub-100ms dwell, so it is dropped at the FIRST of two stacked noise
filters -- the in-kernel `<100ms` guard in `bpf/dwell_monitor.bpf.c`,
before the event ever reaches the ring buffer (the userspace
`if dwell < 1*time.Second` filter in `daemon/controller.go` is the
second). The `events`/`filtered` columns count sessions in-kernel,
*before* that filter, so they make the blind spot directly observable:
events climbs into the thousands while filtered tracks it 1:1 and price
stays 0. An armed, kill-enabled daemon rewrites thousands of files with
price=0 / killed=0 -- the V2.x blind spot, root-caused rather than
merely asserted. Row 1 is that V2-only baseline. When the daemon is run with
`--use-v3-wip` (row 2, 2026-10-08), the rate-based V3 detector (observation
only) *does* register this: `v3_wip` reaches 254 and `v3_price` 369.5 while
the V2 `price` stays 0 — the regression target flipped from blind to
detecting.

## The measured gap

V2.x tracks dwell *latency* and drops short sessions as noise (a
<100ms guard in the kernel, then a <1s guard in userspace), so fast
intermittent encryption never registers -- it is not merely
under-budget, it is filtered out at the source. The `events` column is
counted in-kernel before the filter, so it proves the daemon *saw* the
workload (vs. a dead pipeline): thousands of sessions counted, all
filtered, price unmoved. The attack
row (if present) confirms the same build detects and kills long-dwell
activity. The V3.0 WIP-based (rate) architecture now ships in dual mode
(`--use-v3-wip`, integrated); row 2 above is the flipped regression target —
V3 detects what V2 filters out.
