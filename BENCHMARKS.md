# Dwell-Fiber Benchmarks

Enforcement mode during run: **DRY-RUN (observation only)**

One scenario run against a single daemon instance with default config
(`--alpha=0.5 --budget=5.0`).

| scenario | elapsed | dwell_avg | price | throttled | killed | events | filtered | v3_wip | v3_price |
|----------|--------:|----------:|------:|----------:|-------:|-------:|---------:|-------:|---------:|
| intermittent |     7.3s |  0.00s |  0.000 |        +0 |     +0 |  +2741 |    +2741 |      0 |    0.000 |

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
merely asserted. When the daemon is run with `--use-v3-wip`, the
rate-based V3 detector (observation only) *does* register this: the
`v3_wip`/`v3_price` columns rise while the V2 `price` stays 0 -- the
regression target flipped from blind to detecting.

## The measured gap

V2.x tracks dwell *latency* and drops short sessions as noise (a
<100ms guard in the kernel, then a <1s guard in userspace), so fast
intermittent encryption never registers -- it is not merely
under-budget, it is filtered out at the source. The `events` column is
counted in-kernel before the filter, so it proves the daemon *saw* the
workload (vs. a dead pipeline): thousands of sessions counted, all
filtered, price unmoved. The attack
row (if present) confirms the same build detects and kills long-dwell
activity. The V3.0 WIP-based (rate) architecture is research-in-progress
(unintegrated drafts in `outputs/`, tags v3.0.0-v3.0.2). This
`intermittent` row is the regression baseline any future V3 work must
flip from price~0/killed=0 to detection.
