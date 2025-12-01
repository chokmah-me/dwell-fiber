# Dwell-Fiber V3.0 Migration Status

## Executive Summary

The repository shows **active V3.0 development** but critical components remain **incomplete**. Current state is a **hybrid**: V2.x production code + V3.0 documentation.

---

## ✅ COMPLETED (V3.0)

### Documentation
- ✅ `V3_PIVOT_RESEARCH_DOSSIER.md` - Empirical rationale for WIP metric
- ✅ `V3_MIGRATION.md` - Migration guide (checklist incomplete)
- ✅ `ARCHITECTURE.md` - Updated with V3 layer description
- ✅ `FORMAL_VERIFICATION.md` - Coq proof structure documented
- ✅ `README.md` - References V3.0 architecture (but incorrectly claims "complete")

### Research & Design
- ✅ TCM (Trust Classification Module) tier definitions (T1, T1.5, T2)
- ✅ Weight/budget configurations identified
- ✅ WIP formula: `ω₁·TBW + ω₂·UFM`
- ✅ ADMM price update formula for V3.0

---

## ❌ INCOMPLETE (V3.0)

### Critical Code Gaps

#### 1. **eBPF Program** - MUST REPLACE
**Current**: `bpf/dwell_monitor.bpf.c`
- ❌ Still using `sys_enter_openat`/`sys_enter_close` (V2.x)
- ❌ Measures dwell time, not TBW/UFM
- ❌ No windowed aggregation (1s window required)

**Required**: Switch to `kprobe/vfs_write`
```c
SEC("kprobe/vfs_write")
int track_vfs_write(struct pt_regs *ctx) {
    // Aggregate TBW (bytes written)
    // Track UFM (unique inodes)
    // Emit io_event every 1s window
}
```

**Status**: ⚠️ Draft created in `outputs/dwell_monitor_v3.bpf.c` (needs testing)

---

#### 2. **Go Controller** - MUST REWRITE
**Current**: `daemon/controller.go`
- ❌ Processes `DwellEvent` (PID, duration_ns) - V2.x struct
- ❌ Uses dwell time for price updates
- ❌ No TCM tier classification
- ❌ No WIP calculation

**Required**: Process `WIPEvent` (PID, TBW, UFM)
```go
type WIPEvent struct {
    PID         uint32
    TBW         uint64  // Total Bytes Written
    UFM         uint64  // Unique Files Modified
    TimestampNs uint64
    Comm        [16]byte
}
```

**Status**: ⚠️ Draft created in `outputs/controller_v3.go` (needs integration)

---

#### 3. **BPF Loader** - MUST UPDATE
**Current**: `pkg/bpf/loader.go`
- ❌ Reads `DwellEvent` struct from ring buffer
- ❌ Expects fields: PID, TID, Inode, DurationNs, Filename, Comm

**Required**: Parse `WIPEvent` struct
```go
type WIPEvent struct {
    PID uint32
    TBW uint64
    UFM uint64
    TimestampNs uint64
    Comm [16]byte
}
```

**Status**: ❌ Not started

---

#### 4. **Main Daemon** - MUST REFACTOR
**Current**: `daemon/main.go`
- ❌ Instantiates V2.x controller: `NewController(alpha, budget)`
- ❌ Uses BPFMonitor expecting dwell events

**Required**: Instantiate V3 controller
```go
controllerV3 := NewControllerV3(alpha)
bpfMonitorV3 := NewBPFMonitorV3(controllerV3)
```

**Status**: ❌ Not started

---

#### 5. **Coq Proofs** - MUST EXTEND
**Current**: `coq/dwell_stable.v`
- ❌ Proves convergence for dwell-time metric
- ❌ No lemmas for WIP convexity
- ❌ No tier-switching stability proof

**Required**: Add V3.0 lemmas
```coq
Lemma wip_is_convex : forall ω₁ ω₂ tbw ufm,
  0 <= ω₁ -> 0 <= ω₂ ->
  convex (fun (tbw, ufm) => ω₁ * tbw + ω₂ * ufm).

Lemma dual_price_bounded_under_switch : forall π tier_old tier_new,
  0 <= π ->
  tier_switch tier_old tier_new ->
  0 <= update_price π tier_new.

Lemma bounded_lyapunov_drift_discrete_wip : ...
```

**Status**: ❌ Not started (proofs currently have type errors even for V2.x)

---

## 🚧 BLOCKER ISSUES

### Issue #1: Coq Proofs Don't Compile
**File**: `coq/dwell_stable.v`
**Error**: Type unification error in `dwell_fiber_guarantees` bundled theorem
```
Conjunct 2 expects `d <= budget` after `forall k` (nested)
convergence_to_budget has it before `exists n` (top-level)
```

**Impact**: Cannot verify ANY proofs (V2.x or V3.0) until fixed
**Blocker**: Yes - formal verification is a core feature

---

### Issue #2: V2.x/V3.0 Code Mismatch
**Current State**: 
- Documentation claims "V3 implementation complete"
- Code is 100% V2.x (dwell-time based)
- V3.0 components exist only as drafts in `outputs/`

**Impact**: Confusing for users, blocks V3.0 testing
**Blocker**: Yes - prevents integration work

---

## 📋 V3.0 IMPLEMENTATION CHECKLIST

### Phase 1: Core Component Replacement (Estimated: 8-12 hours)
- [ ] **eBPF Program**
  - [ ] Replace `sys_enter_openat/close` with `kprobe/vfs_write`
  - [ ] Implement 1-second windowed aggregation
  - [ ] Add TBW accumulation logic
  - [ ] Add UFM tracking (inode bitmap)
  - [ ] Update `struct wip_event` definition
  - [ ] Test compilation: `make bpf`
  - [ ] Validate event emission with `bpftool map`

- [ ] **BPF Loader** (`pkg/bpf/loader.go`)
  - [ ] Update `WIPEvent` struct in Go
  - [ ] Update `binary.Read()` to parse new event format
  - [ ] Test ring buffer parsing

- [ ] **Controller** (`daemon/controller.go`)
  - [ ] Replace with `ControllerV3` implementation
  - [ ] Add `ClassifyTier()` (TCM logic)
  - [ ] Add `CalculateWIP()` 
  - [ ] Update `HandleWIPEvent()` to accept TBW/UFM
  - [ ] Update ADMM formula to use tier budgets
  - [ ] Update metrics (rename `dwell_fiber_dwell_time` → `dwell_fiber_wip_current`)

- [ ] **Main Daemon** (`daemon/main.go`)
  - [ ] Instantiate `NewControllerV3()`
  - [ ] Update BPFMonitor to call `HandleWIPEvent()`
  - [ ] Update CLI flags (remove `--budget`, add `--tier-budgets`?)
  - [ ] Update startup banner to show V3.0 mode

### Phase 2: Testing & Validation (Estimated: 4-6 hours)
- [ ] **Unit Tests**
  - [ ] Test TCM tier classification
    - Input: TBW=10GB, UFM=100 → Expected: T1
    - Input: TBW=1GB, UFM=50k → Expected: T1.5
    - Input: TBW=100MB, UFM=10k → Expected: T2
  - [ ] Test WIP calculation
  - [ ] Test ADMM price updates per tier

- [ ] **Integration Tests**
  - [ ] Load V3 eBPF program (requires root)
  - [ ] Generate synthetic I/O workload
    - Backup pattern: High TBW, low UFM
    - Build pattern: High TBW, high UFM
    - Ransomware pattern: Moderate TBW, very high UFM
  - [ ] Verify tier switches logged
  - [ ] Verify WIP metrics exported

- [ ] **Workload Generator** (`test/workload_generator.go`)
  - [ ] Add V3.0 mode: Generate TBW/UFM patterns
  - [ ] `GenerateBackupWorkload()` - T1 pattern
  - [ ] `GenerateBuildWorkload()` - T1.5 pattern
  - [ ] `GenerateRansomwareWorkload()` - T2 pattern with high UFM

### Phase 3: Formal Verification (Estimated: 6-10 hours)
- [ ] **Fix V2.x Coq Proofs** (prerequisite)
  - [ ] Resolve type unification error in `dwell_stable.v`
  - [ ] Use `destruct` pattern for existential quantifiers
  - [ ] Verify `make verify` succeeds

- [ ] **Add V3.0 Coq Lemmas** (`coq/dwell_wip.v` - new file)
  - [ ] `wip_is_convex` - Prove WIP metric is convex
  - [ ] `dual_price_bounded_under_switch` - Price stays ≥ 0 after tier change
  - [ ] `bounded_lyapunov_drift_discrete_wip` - Price drift bounded per window
  - [ ] `tier_switch_stability` - Switching tiers doesn't cause divergence

### Phase 4: Documentation Updates (Estimated: 2-3 hours)
- [ ] **README.md**
  - [ ] Update architecture section with V3.0 details
  - [ ] Add "V3.0 vs V2.x" comparison table
  - [ ] Update build instructions
  - [ ] Add V3.0 status warning

- [ ] **ARCHITECTURE.md**
  - [ ] Replace V2.x data flow with V3.0
  - [ ] Document TCM tier classification logic
  - [ ] Update metrics section (WIP instead of dwell)

- [ ] **V3_MIGRATION.md**
  - [ ] Complete migration checklist (currently empty)
  - [ ] Add before/after code examples
  - [ ] Document breaking changes
  - [ ] Add rollback procedure

- [ ] **CHANGELOG.md**
  - [ ] Add [3.0.0] section with breaking changes
  - [ ] Document removed features (dwell-time metric)
  - [ ] Document new features (WIP, TCM, tier budgets)

### Phase 5: Cleanup (Estimated: 1-2 hours)
- [ ] Remove V2.x artifacts
  - [ ] `bpf/dwell_monitor.bpf.c` (rename to `dwell_monitor_v2.bpf.c.bak`)
  - [ ] `daemon/controller.go` (rename to `controller_v2.go.bak`)
- [ ] Move V3 components from `outputs/` to main paths
- [ ] Update Makefile targets
- [ ] Update CI/CD pipeline (`.github/workflows/ci.yml`)
- [ ] Tag release: `v3.0.0-beta.1`

---

## 🎯 RECOMMENDED ACTION PLAN

### Immediate (This Session)
1. **Fix Coq Proofs** (V2.x) - Unblocks formal verification
   ```bash
   cd coq
   # Apply destruct pattern fix to dwell_stable.v
   make verify
   ```

2. **Create V3.0 Branch** - Isolate development
   ```bash
   git checkout -b feature/v3-wip-metric
   ```

3. **Integrate V3 Controller** - Start with userspace
   ```bash
   cp /mnt/user-data/outputs/controller_v3.go daemon/controller_v3.go
   # Update main.go to conditionally use V3
   ```

### Short-Term (Next 1-2 Days)
4. **Compile V3 eBPF Program**
   ```bash
   cp /mnt/user-data/outputs/dwell_monitor_v3.bpf.c bpf/
   cd bpf && make
   # Fix compilation errors
   ```

5. **Update BPF Loader**
   ```go
   // pkg/bpf/loader.go - add WIPEvent struct
   // Update ring buffer parser
   ```

6. **Integration Testing**
   ```bash
   sudo ./bin/dwell-fiber-daemon --v3-mode
   # Generate workload
   # Verify tier classification
   ```

### Medium-Term (Next Week)
7. **Add Coq V3.0 Proofs**
8. **Update All Documentation**
9. **Comprehensive E2E Testing**
10. **Tag v3.0.0 Release**

---

## 🔧 QUICK FIXES NEEDED NOW

### 1. Update README.md Status Section
```markdown
## Status

- **V2.x**: Production code (daemon/, bpf/) - enforcement live
- **V3.0**: Development in progress - components exist as drafts only
- **Formal Verification**: V2.x proofs have type errors, V3.0 proofs not yet written

⚠️ **Documentation describes V3.0 architecture, but code is still V2.x**
```

### 2. Update V3_MIGRATION.md Checklist
Currently says "all items checked" but they're not implemented:
```markdown
## Migration Checklist

- [ ] Replace open/close hooks with kprobe/vfs_write (IN PROGRESS)
- [ ] Windowed TBW/UFM aggregation (DRAFT ONLY)
- [ ] TCM tier classifier (DRAFT ONLY)
- [ ] ADMM update for WIP/budget (DRAFT ONLY)
- [ ] Coq lemmas for discrete-time WIP (NOT STARTED)
- [ ] Update docs and metrics (PARTIAL)
```

### 3. Fix DEV-NOTES.md
Current status is stale (Nov 6). Update:
```markdown
## 2025-11-15 - V3.0 Integration Status

### Completed
- ✅ V3 eBPF program draft (needs compilation testing)
- ✅ V3 Controller draft (needs integration)
- ✅ Research dossier documenting LockBit problem

### Blocked
- ❌ Coq proofs don't compile (type unification error)
- ❌ V2.x code still in production paths
- ❌ V3 components not integrated into build system

### Next Steps
1. Fix Coq proof compilation errors
2. Create feature branch for V3 work
3. Integrate controller_v3.go into main.go
4. Test eBPF compilation
```

---

## 📊 EFFORT ESTIMATE

| Phase | Tasks | Hours | Difficulty |
|-------|-------|-------|------------|
| Phase 1: Core Code | 7 items | 8-12 | Medium |
| Phase 2: Testing | 3 items | 4-6 | Medium |
| Phase 3: Proofs | 5 items | 6-10 | High |
| Phase 4: Docs | 4 items | 2-3 | Low |
| Phase 5: Cleanup | 4 items | 1-2 | Low |
| **TOTAL** | **23 items** | **21-33 hours** | - |

**Recommended Timeline**: 1-2 weeks for complete V3.0 integration

---

## 🚨 CRITICAL PATH

```
Fix Coq Proofs → Compile V3 eBPF → Update Loader → Integrate Controller → Test → Add V3 Proofs → Release
     [2h]            [4h]             [2h]           [4h]          [6h]      [8h]         [1h]
```

**Minimum Viable V3.0**: ~27 hours of focused work

---

## 📞 CONTACT / COORDINATION

For V3.0 development questions:
- GitHub Issues: Tag with `v3.0-migration`
- Reference: `V3_PIVOT_RESEARCH_DOSSIER.md` for technical rationale

---

**Last Updated**: 2025-11-15  
**Status**: V3.0 development in progress, not production-ready  
**Maintainer**: [@dyb5784](https://github.com/dyb5784)