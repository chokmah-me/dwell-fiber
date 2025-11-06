# Changelog

All notable changes to this project are documented in this file.

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

## [1.3.0] - 2025-11-04

### Added
- **Workload Generator Enhancements**
  - Mode 1: Full test suite (idle/normal/high/critical/varied operations)
  - Mode 2: Continuous workload (long-held files for enforcement testing)
  - Mode 3: Attack simulation (4 stages with 5s→7s→10s→15s dwell escalation)
  - Command-line flags: `-mode`, `-duration`, `-continuous`, `-attack`

- **End-to-End Enforcement**
  - Throttling via cgroups v2 CPU limits (configurable quota, default 20%)
  - Process killing on critical dwell threshold (default 15s+)
  - Graceful shutdown via SIGTERM, fallback to SIGKILL
  - Protected process list (systemd, init, sshd, dbus-daemon, NetworkManager, gdm, Xorg, wayland)

- **Metrics & Observability**
  - Prometheus registry exported at `/metrics` (all gauges: price, dwell, throttled_count, killed_count, enforcement_enabled)
  - Legacy text metrics at `/metrics-basic` for debugging
  - Web UI shows enforcement mode, throttled/killed counts, price/dwell live
  - Daemon startup banner now reflects actual enforcement mode (ENFORCEMENT live vs DRY-RUN)

- **Enforcement Configuration**
  - Tunable thresholds via command-line flags: `--throttle-threshold`, `--kill-threshold`, `--throttle-cpu-quota`
  - Default thresholds: 5s throttle, 15s kill, 20% CPU quota
  - Flags: `--enable-enforcement` (live mode), `--enable-killing` (allow process termination)

### Fixed
- **Process Liveness Check** (critical bug fix)
  - Replaced invalid `os.Signal(nil)` probe with `syscall.Kill(pid, 0)`
  - Treat EPERM as "process exists" (permission denied, not process not found)
  - Resolves false "process no longer exists" errors that blocked all enforcement

- **Throttle Fallback**
  - Replace autogroup write with `syscall.Setpriority` for renice fallback
  - Ensures throttling can apply even if cgroups v2 write fails

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

### Known Limitations
- BPF events fire only on file close (no mid-dwell enforcement yet)
- Mode 2 (continuous, 30s file hold) shows no logs until close—use mode 1 for real-time feedback
- Throttle count shows unique PIDs, not total throttle attempts
- Kill dry-run mode prevents actual termination (can test logic safely)

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
