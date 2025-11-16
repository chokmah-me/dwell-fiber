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

## V3 Commit/Tag/Push Instructions

After updating all files for V3:

```powershell
git add .
git commit -m "feat: Dwell-Fiber V3 – WIP metric, TCM, discrete ADMM, Coq proofs, docs"
git tag -a v3.0.0 -m "Dwell-Fiber V3.0.0: Adaptive I/O Pricing, formal stability"
git push origin main
git push origin --tags
```