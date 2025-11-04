(* Dwell-Fiber Stability Proof - Simplified Version *)
Require Import Reals.
Open Scope R_scope.

Parameter alpha : R.
Parameter budget : R.
Axiom alpha_range : 0 < alpha < 2.
Axiom budget_is_five : budget = 5.

Definition price := R.
Definition dwell := R.

Definition update_price (p : price) (d : dwell) : price :=
  Rmax 0 (p + alpha * (d - budget)).

Theorem price_nonnegative :
  forall (p : price) (d : dwell),
  0 <= p -> 0 <= update_price p d.
Proof.
  intros p d Hp.
  unfold update_price.
  apply Rmax_l.
Qed.

Theorem price_bounded :
  forall (p : price) (d : dwell),
    0 <= p -> 0 <= d <= 100 ->
    0 <= update_price p d.
Proof.
  intros p d Hp Hd.
  apply price_nonnegative.
  assumption.
Qed.

Close Scope R_scope.
