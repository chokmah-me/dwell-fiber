# Dwell-Fiber TODO List

**Last Updated**: 2026-09-24
**Project Version**: v1.7.0
**Status**: No committed roadmap — this is a backlog of ideas, not a release plan. See STATUS.md for the honest state of the project.

---

## 🔴 CRITICAL (Blockers for Next Release)

### Coq Formal Verification Completion
**Status: COMPLETE (2026-09-24)** — 76/76 declarations proven, 0 admitted,
verified under Coq 8.18.0 (`make verify` EXIT 0, `coqchk` clean on all four
modules, fail-closed). Five admitted statements were false as stated and were
corrected (counterexamples documented). Committed as `cac475d`, pushed to
origin `main`.

See `docs/coq_status.md` for the detailed proof-by-proof breakdown.

---

## 🟠 BACKLOG (uncommitted ideas)

Per STATUS.md there is no committed roadmap — the items below are ideas, not promises.

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
**Priority**: Frozen pending external pull — see STATUS.md ("Frozen" / "What's next")

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
