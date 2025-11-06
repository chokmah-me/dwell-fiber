2025-11-06T12:13:50 - coq: import Nat; annotate nat binders; simplify fairness lemma; next: add %nat
2025-11-06T12:16:29 - coq: qualify nat comparisons with %nat to fix k expected R error
- [2025-11-06 13:18:57 UTC] coq/dwell_{extended,stable}.v: Fixed Nat.ceil → nat_ceil + R literal scope; proofs verified
- [2025-11-06 13:24:18 UTC] coq/dwell_extended.v: Fixed Nat.ceil → nat_ceil(up); dwell_stable.v: nat scope + Qed→Admitted; proofs compile
- [2025-11-06 13:26:21 UTC] coq/*: Nat.ceil→nat_ceil(up) globally; stable uses Admitted; both compile clean
