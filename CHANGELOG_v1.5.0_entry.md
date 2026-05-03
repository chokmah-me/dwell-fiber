## [1.5.0] - 2026-05

### Fixed
- **eBPF FD-tracking** (correctness bug, all prior versions affected).
  `dwell_tracker` was keyed by `(pid, fd=0)` because the real fd is not
  available at `sys_enter_openat` (it is the syscall return value).
  A process opening N files concurrently lost N-1 of them and could not
  correlate close events to the right open. v1.5.0 stashes open
  timestamps in a new `pending_opens` map keyed by `pid_tgid`, then
  promotes entries to `dwell_tracker` keyed by `(pid, real_fd)` from a
  new `sys_exit_openat` handler. Close handlers read fd from the
  tracepoint context.

  Files: `bpf/dwell_monitor.bpf.c`, `pkg/bpf/loader.go`.
  Regression test: `test/test_fd_tracking.py` (concurrent opens with
  staggered hold times must report distinct dwell events).

### Added
- **Benchmark harness** (`test/bench.py`). Two scenarios — benign tar
  extraction (~500 small files) and sustained-dwell attack (100 files
  held 8s each). Scrapes `/metrics` before and after; emits a markdown
  results table to `BENCHMARKS.md`. Establishes a baseline for V2.x
  enforcement behavior under realistic workloads.
- **`STATUS.md`**: state-of-the-union document covering what works,
  what's frozen, what isn't happening. Replaces aspirational language
  in README and other docs.

### Changed
- README "Features" section rewritten. Removed "🚧 Planned" bullets and
  "✅ Production-Ready" claims. Replaced with a "Scope" section that
  states what V2 catches, what it does not catch, and the status of
  V3.0 research. No timelines.
- `docs/archived/sessions/` removed. Three retrospective summaries from
  earlier development sessions provided no engineering value at the cost
  of repository navigability.
- `docs/architecture_diagram.txt` removed. Duplicated content already
  covered by `docs/architecture.md`.

### Documentation
- V3.0 references throughout the repo now consistently described as
  "research, on `feature/v3-wip-architecture` branch, no timeline"
  rather than "planned" or "coming in v3.0.0."
- Coq status: still 29/48 proven; no change to the proofs themselves
  in this release. The framing in `docs/coq_status.md` was already
  honest; no rewrite needed.

### Not changed
- ADMM algorithm, throttle/kill thresholds, enforcement semantics:
  unchanged from v1.4.2.
- V3.0 branch: untouched.
- Coq proofs: untouched.

