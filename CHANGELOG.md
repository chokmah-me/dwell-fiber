# Changelog

All notable changes to this project are documented in this file.

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
