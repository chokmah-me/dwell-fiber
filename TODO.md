# Dwell-Fiber TODO List

**Last Updated**: 2025-12-30
**Project Version**: v1.4.2
**Status**: Production-ready with ongoing enhancements

---

## 🔴 CRITICAL (Blockers for Next Release)

### Coq Formal Verification Completion
**Priority: MEDIUM | Estimated: 18-24 hours remaining**
**Current Status**: 60% complete (29/48 proofs), 19 admitted

See `docs/coq_status.md` for detailed proof-by-proof breakdown.

#### Phase 1: Core Stability Proofs (dwell_stable.v) - 6 remaining
- [x] Complete `price_nonnegative` proof ✅ PROVEN
- [x] Complete `price_bounded` proof ✅ PROVEN
- [ ] Complete `convergence_to_budget` proof (requires Banach fixed-point)
- [ ] Complete `liveness_normal_mode` proof
- [ ] Complete `liveness_attack_mode` proof
- [ ] Complete `no_starvation` proof
- [ ] Complete `ransomware_detection` proof
- [ ] Complete `dwell_fiber_guarantees` bundled theorem

**Files**: `coq/dwell_stable.v` (50% complete)

#### Phase 2: Resilience Proofs (dwell_kernel_resilience.v) - 4 remaining
- [x] Complete `update_price_monotonic` proof ✅ PROVEN
- [ ] Complete `bounded_loss_preserves_dwell_bound` - ≥(1-δ) retention proof
- [ ] Complete `price_update_monotonic_dwell` - Stream monotonicity proof
- [ ] Complete `bounded_price_under_loss` - Combine lemmas
- [ ] Complete `admm_resilience_to_event_loss` - Main theorem (HIGH difficulty)
- [ ] Complete `resilience_example` - Concrete instantiation

**Files**: `coq/dwell_kernel_resilience.v`

#### Phase 3: Extended Properties (dwell_extended.v) - 10-15 hours
- [ ] Complete `liveness_attack_eventually_detected` (temporal logic - HIGH difficulty)
- [ ] Complete `fairness_benign_not_throttled`
- [ ] Complete `attack_resistance_rapid_encryption`
- [ ] Complete `safety_protected_processes_never_killed`
- [ ] Complete `convergence_discrete_time_admm`
- [ ] Complete `multi_process_fairness`
- [ ] Complete `bounded_false_positive_rate`

**Files**: `coq/dwell_extended.v`

---

## 🟠 HIGH PRIORITY (V1.5.0 Features)

### Mid-Dwell Enforcement Timer
**Estimated: 6-8 hours**

**Goal**: Detect ransomware WHILE file is open (not just on close)

**Approach**: Periodic timer (5s interval) checks dwell duration

**Files to modify**:
- `bpf/dwell_monitor.bpf.c` - Add timer callback
- `daemon/controller.go` - Handle mid-dwell events

**Test**: Hold file open for 30s, verify enforcement triggers before close

### Throttle Attempt Counter
**Estimated: 1-2 hours**

**Goal**: Track total throttle attempts (not just unique PIDs)

**Files**:
- `pkg/enforcement/throttler.go` - Increment counter on attempt
- `daemon/metrics.go` - Register `dwell_fiber_throttle_attempts` gauge

### Performance Profiling
**Estimated: 2-3 hours**

**Goal**: Identify bottlenecks with pprof

**Files**:
- `daemon/main.go` - Add pprof HTTP handler on port 6060

### Integration Tests with Real Workloads  
**Estimated: 3-5 hours**

**Goal**: Test with actual backup tools (rsync, tar, gcc)

**Files to create**:
- `test/integration/backup_test.go`
- `test/integration/build_test.go`

---

## 🟡 MEDIUM PRIORITY (Improvements)

### Documentation Improvements - 4-6 hours

- [ ] Reorganize into subdirectories (docs/user/, docs/development/, docs/coq/)
- [ ] Create SECURITY.md with vulnerability reporting policy
- [ ] Improve README.md quick start with prerequisites check script
- [ ] Create docs/development/api.md for Prometheus metrics format

### Code Quality - 6-10 hours

- [ ] Add Godoc comments for all exported functions in `pkg/`
- [ ] Increase test coverage to 80%+ (priority: throttler.go, controller.go)

### Build System - 2-4 hours

- [ ] Add `make install` target (install binaries to /usr/local/bin)
- [ ] Add cross-compilation support (GOOS/GOARCH targets)
- [ ] Docker containerization (requires privileged mode for eBPF)

---

## 🟢 LOW PRIORITY (Future Work)

### V2.0.0 Production Hardening - 20-30 hours

- [ ] Third-party security audit
- [ ] SELinux/AppArmor profiles (`selinux/dwell-fiber.te`, `apparmor/dwell-fiber`)
- [ ] Systemd hardening (CapabilityBoundingSet, ProtectSystem, etc.)
- [ ] Real-world ransomware testing (in sandbox - Cuckoo)
- [ ] Performance benchmarking at scale (1000+ processes)

### V3.0 WIP Architecture

**Status**: Unintegrated drafts in `outputs/`, preserved at tags `v3.0.0`–`v3.0.2` (no active branch)  
**Checklist**: See `docs/v3-roadmap.md`  
**Estimated**: 21-33 hours  
**Priority**: Deferred until v1.5.0 complete

---

## ✅ COMPLETED

### v1.4.2 (December 30, 2025)
- [x] Coq proof compilation fixes (60% proofs complete - 29/48)
- [x] README refactor (304 → 165 lines)
- [x] Created docs/installation.md, v2-architecture.md, v3-roadmap.md
- [x] Documentation staleness cleanup

### v1.4.0-v1.4.1 (December 30, 2025)
- [x] Repository cleanup and reorganization
- [x] Documentation truth correction
- [x] V3 materials moved to feature branch
- [x] Session file consolidation
- [x] Coq status documentation created

### v1.3.0 (November 4, 2025)
- [x] Enforcement system (throttle + kill)
- [x] Safety checks (protected processes)
- [x] Metrics export (Prometheus)
- [x] Web dashboard
- [x] Workload generator (3 modes)
- [x] BPF event processing fix

---

## How to Use This TODO

### For Contributors
1. Pick a task from CRITICAL or HIGH priority
2. Create a GitHub issue referencing this TODO item
3. Create a feature branch: `feature/todo-item-name`
4. Submit PR with tests

### For Maintainers
- Update completion status as work progresses
- Add new items as they arise
- Move completed items to COMPLETED section
- Tag GitHub issues with priority labels

---

**Maintainer**: [@chokmah-me](https://github.com/chokmah-me)  
**Last Review**: 2025-12-30
