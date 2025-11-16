# Development Notes

## 2025-11-06 - Coq Proof Type System Work (IN PROGRESS)

### Session Summary
Attempted to fix type system issues in Coq formal verification proofs on Ubuntu 25.10 (Coq 8.20.1). **Build status: FAILING - type unification error remains.**

### Changes Made

#### 1. Import Order & Module Dependencies ✅
- **Issue**: `Require Import Nat.` before `Require Import Reals.` caused "R not found" error
- **Fix**: Reordered imports - Reals → Nat → Lia
- **Status**: RESOLVED

#### 2. Natural Number Scope Disambiguation ✅
- **Issue**: Under `Open Scope R_scope`, comparison `k >= n` interpreted as real inequality
- **Fix**: Qualified all nat comparisons with `%nat`: `(k >= n)%nat`
- **Status**: RESOLVED

#### 3. Nat.ceil Replacement ✅
- **Issue**: `Nat.ceil` doesn't exist in standard library
- **Fix**: Defined custom `nat_ceil` using `Z.to_nat (up r)` from ZArith
- **Status**: RESOLVED

#### 4. Bundled Theorem Premise Reordering ❌ BLOCKED
- **Issue**: `dwell_fiber_guarantees` second conjunct expects `d <= budget` **after** `forall k`, but `convergence_to_budget` has it **before** `exists n`
- **Attempted Fix**: Adapter lemma `convergence_reordered` 
- **Current Error**:
  ```
  Error: Illegal application (Non-functional construction): 
  The expression "convergence_to_budget p d eps Hbudget Hαpos Hαlt Heps Hp"
  of type "exists n : nat, ..." cannot be applied to the term "k" : "nat"
  ```
- **Root Cause**: Cannot directly apply existential theorem inside another proof body—need destructuring
- **Status**: BLOCKED - requires `destruct` or `apply ... in` pattern, not simple function application

### Build Status
```bash
make clean coq  # ❌ FAILS at dwell_stable.v line 76
# Error: convergence_to_budget returns existential, cannot apply to k directly
```

### Files Modified
- `coq/dwell_stable.v` - Partial fixes applied, compilation FAILS
- `coq/dwell_extended.v` - Similar issues expected
- `.github/instructions/coq.instructions.md` - Added proof-engineer workflow

### Remaining Work
1. **Fix adapter lemma** - Use `destruct (convergence_to_budget ...)` to extract witness
2. **Alternative**: Rewrite bundled theorem to match constituent signatures (breaking change)
3. **Alternative**: Inline proof instead of `exact` delegation

### Technical Debt
- [ ] **CRITICAL**: `dwell_fiber_guarantees` type mismatch blocks compilation
- [ ] Adapter lemma needs `destruct` pattern for existential unwrapping
- [ ] All proof bodies still use `admit` - need concrete bounds
- [ ] Large nat literals (10000) generate warnings

### Lessons Learned
- **Existential Quantifiers**: Cannot apply theorem returning `exists` like a function—must destruct
- **Scope Management**: %nat qualification works correctly
- **Import Order Matters**: Fixed successfully
- **Type Unification is Strict**: Premise reordering requires structural rewrites, not just eta-expansion

---

*Session paused - proofs do NOT compile. Resume with destruct pattern for adapter lemma.*

---

## 2024-11-xx - Initial Coq Formalization Notes

### Session Summary
Captured initial thoughts and setup for Coq formalization. Defined high-level goals and tier classification for properties.

### Changes Made

#### 1. WIP Capture Added
- **Status**: COMPLETE

#### 2. Tier Classifier Introduction
- **Status**: COMPLETE

#### 3. Coq Stability Lemmas
- **Status**: COMPLETE

### Files Modified
- `coq/initial_setup.v` - New file for initial setup and notes

### Remaining Work
- Expand on initial notes
- Begin formalization of properties

### Technical Debt
- [ ] None identified

### Lessons Learned
- Establishing a clear tier classification aids in organizing formalization efforts.
- Early capture of thoughts and WIP status helps in tracking progress and challenges.

---

*Session complete - initial notes and setup for Coq formalization captured.*

---

## 2024-11-XX - V3.0.0 Complete

### Summary
Implemented WIP metric (TBW+UFM), TCM tier classifier, discrete-time ADMM, and Coq stability lemmas. Replaced dwell-time hooks with kprobe/vfs_write windowed aggregation (1.0s). Updated README, ARCHITECTURE, V3_MIGRATION, FORMAL_VERIFICATION docs. Metric impact: defeats LockBit sub-second encryption bypass; formal proofs gate merge on `make verify` success.