# Formal Verification: Coq Proof Structure

## Definitions

- WIP: `wip (ω₁ ω₂ tbw ufm) := ω₁ * tbw + ω₂ * ufm`
- Tier weights/budgets match daemon config.
- ADMM update: `price' = max(0, price + α·(WIP - budget))`

## Lemmas

1. **wip_is_convex**: WIP is convex in TBW/UFM.
2. **dual_price_bounded_under_switch**: Price stays ≥ 0 under tier switch.
3. **bounded_lyapunov_drift_discrete_wip**: Price drift per window is bounded.

## Build

```bash
make verify
```
Proofs currently fail to compile due to type unification errors (see DEV-NOTES.md). Must be fixed before merge.
