# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### In Progress
- V3.0 WIP-based architecture integration
- Mid-dwell enforcement timer implementation

## [1.4.0] - 2025-12-01

### ✅ Coq Formal Verification COMPLETE

**Major Achievement**: All Coq proofs now compile and verify successfully!

#### Compilation Fixes (dwell_stable.v)
- ✅ Fixed compound inequality syntax (`0 < alpha < 2` → separate axioms)
- ✅ Added missing imports for `Rmax` and `Rabs` (Max, RIneq modules)
- ✅ Fixed let-binding syntax in theorem statements
- ✅ Corrected `nat_ceil` type conversion implementation
- ✅ Completed incomplete proof applications with proper tactics
- ✅ **Result**: `dwell_stable.v` compiles without errors

#### New: Kernel-Userspace Resilience Model
- ✅ Created `coq/dwell_kernel_resilience.v` (335 lines)
- ✅ Formalized bounded event loss model with δ-rate constraints
- ✅ **Lemma 1**: `bounded_loss_preserves_dwell_bound`
  - Proves ≥ (1-δ) fraction of dwell retained under loss
  - Uses induction and inequality reasoning
- ✅ **Lemma 2**: `price_update_monotonic_dwell`
  - Shows price updates are monotonic in total dwell
  - Handles `Rmax` case analysis correctly
- ✅ **Lemma 3**: `bounded_price_under_loss`
  - Proves price remains bounded and non-divergent
  - Critical for system stability guarantee
- ✅ **Main Theorem**: `admm_resilience_to_event_loss`
  - Combines lemmas to show bounded deviation under loss

#### Integration & Testing
- ✅ Updated `coq/Makefile` with new file dependencies
- ✅ Updated root `Makefile` verification target
- ✅ Created `coq/test_resilience.v` (22 unit tests)
- ✅ Verified Go controller parameter alignment
- ✅ Designed `max_burst_loss = 5` test case
- ✅ Created comprehensive integration verification report

#### Documentation
- ✅ `COQ_INSTALLATION.md` - Complete installation guide
- ✅ `COQ_FIX_ANALYSIS.md` - Compilation error analysis
- ✅ `COQ_RESILIENCE_STRATEGY.md` - Resilience model strategy
- ✅ `COQ_RESILIENCE_IMPLEMENTATION.md` - Implementation guide
- ✅ `COQ_INTEGRATION_GUIDE.md` - Integration framework
- ✅ `INTEGRATION_VERIFICATION_REPORT.md` - Go/Coq integration analysis

### Security & Resilience
- **Event Loss Tolerance**: System proven stable with up to 10% eBPF event loss
- **Burst Loss Handling**: Verified with `max_burst_loss = 5` constraint
- **Formal Guarantees**: Three lemmas provide deterministic bounds on system behavior
- **Integration**: Go controller parameters align with Coq model

### Performance Impact
- No runtime performance impact (verification is compile-time)
- Resilience model adds compile-time proofs, no overhead
- System maintains all v1.3.0 performance characteristics

### Breaking Changes
- None - fully backward compatible with v1.3.0
- All changes are additive (new proofs, tests, documentation)

### Migration from v1.3.0
```bash
# No code changes required
# Simply pull latest and rebuild
git pull origin main
make clean daemon

# Verify proofs (requires Coq 9.1+)
cd coq && make verify
```

### Known Limitations
- Coq must be installed to verify proofs (see COQ_INSTALLATION.md)
- Windows users may need to configure OPAM PATH (see troubleshooting guide)
- V3.0 WIP-based architecture still in development (separate effort)

---

## [3.0.0] – 2024-11-XX

### Added
- Weighted I/O Pressure (WIP) metric: TBW + UFM
- Trust Classification Module (TCM) tiers
- Discrete-time ADMM controller
- eBPF kprobe/vfs_write windowed aggregation
- Formal Coq proofs for stability
- Documentation: ARCHITECTURE.md, V3_MIGRATION.md, FORMAL_VERIFICATION.md

### Changed
- daemon/dwell_user.go: handles io_event (TBW/UFM)
- bpf/dwell_monitor.bpf.c: switched hooks
- Metrics: dwell_wip_current, dwell_price, dwell_tier_switches

### Removed
- Old dwell-time logic, open/close hooks

### Security
- Defeats intermittent-access ransomware

### Performance
- eBPF overhead <5%, ringbuf latency <1ms

### Testing
- Unit, formal, integration, E2E

---

## [1.3.0] - 2025-11-04

### Added
- **End-to-End Enforcement System**
  - Throttling via cgroups v2 CPU limits (configurable quota, default 20%)
  - Process killing on critical dwell threshold (default 15s+)
  - Graceful shutdown via SIGTERM, fallback to SIGKILL
  - Protected process list (systemd, init, sshd, dbus-daemon, NetworkManager, gdm, Xorg, wayland)
  - Safety checks: liveness detection, self-protection, cannot harm critical processes

- **Workload Generator Enhancements**
  - Mode 1: Full test suite (idle/normal/high/critical/varied operations)
  - Mode 2: Continuous workload (long-held files for enforcement testing)
  - Mode 3: Attack simulation (4 stages with 5s→7s→10s→15s dwell escalation)
  - Command-line flags: `-mode`, `-duration`, `-continuous`, `-attack`

- **Metrics & Observability**
  - Prometheus registry exported at `/metrics` (gauges: price, dwell, throttled_count, killed_count, enforcement_enabled)
  - Legacy text metrics at `/metrics-basic` for debugging
  - Web UI shows enforcement mode, throttled/killed counts, price/dwell live
  - Daemon startup banner reflects actual enforcement mode (ENFORCEMENT live vs DRY-RUN)

- **Enforcement Configuration**
  - Tunable thresholds: `--throttle-threshold`, `--kill-threshold`, `--throttle-cpu-quota`
  - Default thresholds: 5s throttle, 15s kill, 20% CPU quota
  - Flags: `--enable-enforcement` (live mode), `--enable-killing` (allow termination)

### Fixed
- **Process Liveness Check** (critical bug fix)
  - Replaced invalid `os.Signal(nil)` probe with `syscall.Kill(pid, 0)`
  - Treat EPERM as "process exists" (permission denied, not process not found)
  - Resolves false "process no longer exists" errors that blocked all enforcement

- **Throttle Fallback**
  - Replaced autogroup write with `syscall.Setpriority` for renice fallback
  - Ensures throttling applies even if cgroups v2 write fails

- **Enforcement Banner**
  - Shows actual mode based on flags/config, not hardcoded DRY-RUN
  - Prints thresholds and protected processes on startup

- **Throttle Count Tracking**
  - Timestamp always updates on successful throttle to prevent dedup confusion
  - Count reflects unique throttled PIDs at any moment

### Tested
- Mode 1: Multiple rapid file operations, throttling applies within seconds
- Mode 3: Attack simulation with 4 stages; stages 2-3 throttled, stage 4 killed gracefully
- Cgroups v2: CPU quota verified (`/sys/fs/cgroup/dwell-fiber.slice/cpu.max`)
- Metrics: Prometheus scrape succeeds, gauges update on close events
- Protected processes: System daemons cannot be throttled/killed
- Dry-run mode: Can disable enforcement without code changes

### Performance
- BPF Events: 2500+/minute (real kernel monitoring)
- Throttle Actions: 12+ verified
- Kill Actions: Successful at 15s threshold
- Controller Latency: <10ms
- Memory Usage: ~5MB daemon
- CPU Usage: <1% idle, spikes during throttling

### Known Limitations
- BPF events fire only on file close (no mid-dwell enforcement yet)
- Mode 2 (continuous, 30s file hold) shows no logs until close—use mode 1 for real-time feedback
- Throttle count shows unique PIDs, not total throttle attempts

---

## [0.2.0] - 2025-11-06

### Security
- **BREAKING: Enforcement now OFF by default** (safe-by-default model)
  - Was: Enforcement always active (risk of accidental production impact)
  - Now: Observation mode by default, explicit `--enable-enforcement` to activate
  - Impact: Prevents surprise throttling/killing on production systems
  - Migration: Add `--enable-enforcement` flag to re-enable enforcement

### Fixed
- **Enforcer enabled flag hardcoded to true** (ISSUE #3)
  - `daemon/controller.go`: Removed `enfConfig.Enabled = true` in `NewController()`
  - Now respects CLI flags: `--enable-enforcement`, `--enable-killing`
  - Metrics correctly report enforcement mode (0=dry-run, 1=enabled)

- **Setpriority undefined syscall** (ISSUE #2)
  - `pkg/enforcement/throttler.go`: Refactored to use platform-specific handler
  - `pkg/enforcement/throttler_linux.go`: New file with `golang.org/x/sys/unix.Setpriority`
  - Build tag `//go:build linux` ensures cross-platform compatibility
  - Fallback throttling via nice adjustment works correctly

- **eBPF inode tracking always 0** (ISSUE #1)
  - `bpf/dwell_monitor.bpf.c`: Changed key from `(PID, inode)` to `(PID, FD)`
  - File descriptor available at open/close; inode requires kernel data structure walk
  - Now supports multiple concurrent files per PID
  - Fixes: process opening 2+ files simultaneously report correct dwell times

- **Stale BPF map entries leak memory** (ISSUE #4)
  - Added `pid_activity` map for last-seen timestamp per PID
  - Prevents indefinite accumulation of stale entries from crashed processes
  - Enables future cleanup implementation (next release)

- **No noise filtering** (ISSUE #5)
  - Added 100ms minimum dwell threshold at eBPF close handler
  - Reduces spurious events from very short-lived file opens (standard tool overhead)

### Changed
- Enforcement thresholds now OFF by default:
  - Old: `--enable-enforcement` (soft dry-run, metrics collected, no actual throttling)
  - New: No enforcement unless `--enable-enforcement` flag provided
  - `enforcementMode` metric: 0 = observation, 1 = enforcement enabled
  - CLI behavior documented in `USER_GUIDE.md`

- eBPF key structure updated for better tracking:
  - `struct dwell_key` now uses `__u32 fd` instead of `__u64 inode`
  - Simplifies open→close correlation
  - Reduces BPF map pressure for single-file processes

### Tested
- Ubuntu 25.10 (kernel 6.17)
- Go 1.24.9
- Coq 8.18+
- clang-20 with libbpf-dev
- CI: GitHub Actions with updated ubuntu-latest workflow

### Migration Guide
```bash
# Old behavior (enforcement always on):
sudo ./bin/dwell-fiber-daemon

# New safe default (observation only):
sudo ./bin/dwell-fiber-daemon

# Re-enable enforcement (if needed):
sudo ./bin/dwell-fiber-daemon --enable-enforcement

# With throttling but no killing:
sudo ./bin/dwell-fiber-daemon --enable-enforcement --budget=5.0
```

---

## [1.2.0] - 2025-11-04

- Enhanced workload generator with continuous and attack simulation modes
- Fixed daemon enforcement banner to reflect real config

---

## [1.1.0] - 2025-11-03

- Enforcement framework with throttling and killing pipeline
- Safety checks and protected process list
- Test suite with 6 scenarios

---

## [1.0.0] - 2025-11-01

- Initial simulation mode with ADMM price algorithm
- BPF event capture and ring buffer processing
- Metrics server and web dashboard
- Coq formal verification proofs

---

## Past

- 2025-10-30 — docs: comprehensive documentation, citations, CHANGELOG sanitization
- 2025-10-29 — docs: README cleanup and normalization
