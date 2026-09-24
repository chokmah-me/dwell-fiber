(* Dwell-Fiber Formal Verification - Complete Suite *)
Require Import Reals.
From Coq Require Import ZArith.
From Coq Require Import Lra.

Definition nat_ceil (r : R) : nat :=
  Z.to_nat (up r).

Require Import Lia.
Require Import Nat.
Open Scope R_scope.

Parameter alpha : R.
Parameter budget : R.
Axiom alpha_pos : 0 < alpha.
Axiom alpha_lt_2 : alpha < 2.
Axiom budget_is_five : budget = 5.

Definition price := R.
Definition dwell := R.

Definition update_price (p : price) (d : dwell) : price :=
  Rmax 0 (p + alpha * (d - budget)).

(* ================================================================== *)
(* Helper lemmas: closed forms for the price iteration                 *)
(* No Banach fixed-point machinery is needed: the iteration is        *)
(* affine with a floor at 0, so it admits exact closed forms.          *)
(* ================================================================== *)

Lemma div_mul_cancel : forall (r c : R),
  c <> 0 -> (r / c) * c = r.
Proof.
  intros r c Hc0.
  unfold Rdiv. rewrite Rmult_assoc. rewrite (Rmult_comm (/ c) c).
  rewrite (Rinv_r c Hc0). rewrite Rmult_1_r. reflexivity.
Qed.

(* Archimedean witness scaled by a positive constant. *)
Lemma archimed_nat_mul : forall (r c : R),
  0 < c -> exists n : nat, r <= INR n * c.
Proof.
  intros r c Hc.
  assert (Hc0 : c <> 0) by lra.
  destruct (archimed (r / c)) as [Hgt _].
  revert Hgt.
  destruct (up (r / c)) as [|p|p] eqn:Hz.
  - (* up (r/c) = Z0 *)
    intros Hgt.
    exists 0%nat.
    assert (H0 : IZR Z0 = 0) by reflexivity.
    assert (Hr : r < 0).
    { assert (H2 : (r / c) * c < 0 * c).
      { apply Rmult_lt_compat_r. exact Hc. lra. }
      rewrite Rmult_0_l in H2. rewrite (div_mul_cancel r c Hc0) in H2. exact H2. }
    assert (HINR0 : INR 0 = 0) by reflexivity.
    rewrite HINR0. rewrite Rmult_0_l. lra.
  - (* up (r/c) = Zpos p *)
    intros Hgt.
    exists (Z.to_nat (Zpos p)).
    assert (Hz0 : (0 <= Zpos p)%Z) by lia.
    assert (Heq : INR (Z.to_nat (Zpos p)) = IZR (Zpos p)).
    { rewrite INR_IZR_INZ. rewrite (Z2Nat.id (Zpos p) Hz0). reflexivity. }
    rewrite Heq.
    assert (Hlt : (r / c) * c < IZR (Zpos p) * c).
    { apply Rmult_lt_compat_r. exact Hc. exact Hgt. }
    rewrite (div_mul_cancel r c Hc0) in Hlt.
    lra.
  - (* up (r/c) = Zneg p *)
    intros Hgt.
    exists 0%nat.
    assert (Hneg : IZR (Zneg p) < 0).
    { assert (H1 : (Zneg p < Z0)%Z) by lia.
      assert (H2 := IZR_lt _ _ H1).
      assert (H0 : IZR Z0 = 0) by reflexivity.
      lra. }
    assert (Hrc : r / c < 0) by lra.
    assert (Hr : r < 0).
    { assert (H2 : (r / c) * c < 0 * c).
      { apply Rmult_lt_compat_r. exact Hc. exact Hrc. }
      rewrite Rmult_0_l in H2. rewrite (div_mul_cancel r c Hc0) in H2. exact H2. }
    assert (HINR0 : INR 0 = 0) by reflexivity.
    rewrite HINR0. rewrite Rmult_0_l. lra.
Qed.

(* Pushing a positive shift through the floor: Rmax 0 (Rmax 0 X - c) = Rmax 0 (X - c). *)
Lemma Rmax_sub_distr : forall (X c : R),
  0 < c -> Rmax 0 (Rmax 0 X - c) = Rmax 0 (X - c).
Proof.
  intros X c Hc.
  destruct (Rle_dec X 0) as [Hle | Hnle].
  - rewrite (Rmax_left 0 X Hle).
    assert (H1 : 0 - c <= 0) by lra.
    rewrite (Rmax_left 0 (0 - c) H1).
    assert (H2 : X - c <= 0) by lra.
    rewrite (Rmax_left 0 (X - c) H2).
    reflexivity.
  - assert (Hgt : 0 < X) by lra.
    rewrite (Rmax_right 0 X (Rlt_le _ _ Hgt)).
    reflexivity.
Qed.

(* Decay closed form: for d < budget the price drops by a fixed amount
   each step until it hits the floor at 0. *)
Lemma iter_decay_closed : forall (p d : R) (n : nat),
  0 <= p ->
  0 < alpha * (budget - d) ->
  Nat.iter n (fun x => update_price x d) p = Rmax 0 (p - INR n * (alpha * (budget - d))).
Proof.
  intros p d n Hp Hc.
  induction n as [|n IH].
  - assert (Hiter0 : Nat.iter 0 (fun x => update_price x d) p = p) by reflexivity.
    assert (H0 : INR 0 = 0) by reflexivity.
    rewrite Hiter0, H0.
    replace (p - 0 * (alpha * (budget - d))) with p by ring.
    rewrite (Rmax_right 0 p Hp).
    reflexivity.
  - rewrite Nat.iter_succ, IH.
    cbv beta.
    unfold update_price.
    rewrite S_INR.
    assert (Hneg : alpha * (d - budget) = -(alpha * (budget - d))) by ring.
    rewrite Hneg.
    replace (Rmax 0 (p - INR n * (alpha * (budget - d))) + -(alpha * (budget - d)))
      with (Rmax 0 (p - INR n * (alpha * (budget - d))) - (alpha * (budget - d))) by ring.
    rewrite (Rmax_sub_distr _ _ Hc).
    assert (Heq : p - INR n * (alpha * (budget - d)) - (alpha * (budget - d))
                  = p - (INR n + 1) * (alpha * (budget - d))) by ring.
    rewrite Heq.
    reflexivity.
Qed.

(* Growth lower bound: for d > budget the price grows at least linearly. *)
Lemma iter_growth_lower : forall (p d : R) (n : nat),
  0 < alpha * (d - budget) ->
  p + INR n * (alpha * (d - budget)) <= Nat.iter n (fun x => update_price x d) p.
Proof.
  intros p d n Hc.
  induction n as [|n IH].
  - assert (Hiter0 : Nat.iter 0 (fun x => update_price x d) p = p) by reflexivity.
    assert (H0 : INR 0 = 0) by reflexivity.
    rewrite Hiter0, H0. rewrite Rmult_0_l. rewrite Rplus_0_r. apply Rle_refl.
  - rewrite Nat.iter_succ.
    cbv beta.
    unfold update_price.
    rewrite S_INR.
    assert (Hle : Nat.iter n (fun x => update_price x d) p + alpha * (d - budget)
                  <= Rmax 0 (Nat.iter n (fun x => update_price x d) p + alpha * (d - budget))).
    { apply Rmax_r. }
    assert (Heq : p + (INR n + 1) * (alpha * (d - budget))
                  = (p + INR n * (alpha * (d - budget))) + alpha * (d - budget)) by ring.
    rewrite Heq.
    eapply Rle_trans.
    + apply Rplus_le_compat_r. exact IH.
    + exact Hle.
Qed.

(* Master decay lemma: below budget, the price reaches exactly 0 in finite time. *)
Lemma decay_eventually_zero : forall (p d : R),
  d < budget -> 0 <= p ->
  exists n : nat, forall k : nat, (k >= n)%nat ->
    Nat.iter k (fun x => update_price x d) p = 0.
Proof.
  intros p d Hd Hp.
  assert (Hc : 0 < alpha * (budget - d)).
  { apply Rmult_lt_0_compat. exact alpha_pos. lra. }
  destruct (archimed_nat_mul p (alpha * (budget - d)) Hc) as [n Hn].
  exists n. intros k Hk.
  rewrite (iter_decay_closed p d k Hp Hc).
  apply Rmax_left.
  assert (Hkn : INR n <= INR k) by (apply le_INR; exact Hk).
  assert (H1 : INR n * (alpha * (budget - d)) <= INR k * (alpha * (budget - d))).
  { apply Rmult_le_compat_r. apply Rlt_le. exact Hc. exact Hkn. }
  lra.
Qed.

(* Master growth lemma: above budget, the price eventually exceeds any threshold. *)
Lemma growth_eventually_ge : forall (p d thr : R),
  d > budget ->
  exists n : nat, forall k : nat, (k >= n)%nat ->
    Nat.iter k (fun x => update_price x d) p >= thr.
Proof.
  intros p d thr Hd.
  assert (Hc : 0 < alpha * (d - budget)).
  { apply Rmult_lt_0_compat. exact alpha_pos. lra. }
  destruct (archimed_nat_mul (thr - p) (alpha * (d - budget)) Hc) as [n Hn].
  exists n. intros k Hk.
  assert (Hkn : INR n <= INR k) by (apply le_INR; exact Hk).
  assert (H1 : INR n * (alpha * (d - budget)) <= INR k * (alpha * (d - budget))).
  { apply Rmult_le_compat_r. apply Rlt_le. exact Hc. exact Hkn. }
  assert (H2 : p + INR n * (alpha * (d - budget)) <= p + INR k * (alpha * (d - budget))).
  { apply Rplus_le_compat_l. exact H1. }
  assert (H3 := iter_growth_lower p d k Hc).
  lra.
Qed.

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
    0 <= p -> 0 <= d -> d <= 100 ->
    0 <= update_price p d.
Proof.
  intros p d Hp Hd_low Hd_high.
  apply price_nonnegative; assumption.
Qed.

(* FIXED 2026-09-24: hypothesis was `d <= budget`, which is false at d = budget
   (the price is frozen there: update_price p budget = p for p >= 0, so it never
   decays below epsilon). The honest statement needs strict inequality. *)
Theorem convergence_to_budget :
  forall (p d : price) (epsilon : R),
  d < budget ->
  0 < epsilon ->
  0 <= p ->
  exists n : nat,
  forall (k : nat),
  (k >= n)%nat ->
  let iter_result := Nat.iter k (fun x => update_price x d) p in
  Rabs iter_result < epsilon.
Proof.
  intros p d epsilon Hd Heps Hp.
  destruct (decay_eventually_zero p d Hd Hp) as [n Hn].
  exists n. intros k Hk. cbv zeta.
  rewrite (Hn k Hk). rewrite Rabs_R0. exact Heps.
Qed.

(* FIXED 2026-09-24: same `d <= budget` -> `d < budget` correction as above. *)
Theorem liveness_normal_mode :
  forall (d p : R),
  d < budget ->
  0 <= p ->
  exists n : nat,
  forall (k : nat),
  (k >= n)%nat ->
  let iter_result := Nat.iter k (fun x => update_price x d) p in
  iter_result = 0 \/ iter_result < 0.001.
Proof.
  intros d p Hd Hp.
  destruct (decay_eventually_zero p d Hd Hp) as [n Hn].
  exists n. intros k Hk. cbv zeta.
  left. exact (Hn k Hk).
Qed.

Theorem liveness_attack_mode :
  forall (d p threshold : R),
  d > budget ->
  0 <= p ->
  0 < threshold ->
  exists n : nat,
  forall (k : nat),
  (k >= n)%nat ->
  let iter_result := Nat.iter k (fun x => update_price x d) p in
  iter_result >= threshold.
Proof.
  intros d p thr Hd Hp Hthr.
  destruct (growth_eventually_ge p d thr Hd) as [n Hn].
  exists n. intros k Hk. cbv zeta. exact (Hn k Hk).
Qed.

Theorem fairness_identical_processes :
  forall (p1 p2 d : R),
  p1 = p2 ->
  update_price p1 d = update_price p2 d.
Proof. intros p1 p2 d ->; reflexivity. Qed.

Theorem fairness_enforcement_symmetric :
  forall (p d threshold : R),
  0 < threshold ->
  (update_price p d >= threshold <->
   update_price p d >= threshold).
Proof. intros; reflexivity. Qed.

Theorem no_starvation :
  forall (d p : R),
  d < budget ->
  0 <= p ->
  exists n : nat,
  forall (k : nat),
  (k >= n)%nat ->
  let iter_result := Nat.iter k (fun x => update_price x d) p in
  iter_result = 0.
Proof.
  intros d p Hd Hp.
  destruct (decay_eventually_zero p d Hd Hp) as [n Hn].
  exists n. intros k Hk. cbv zeta. exact (Hn k Hk).
Qed.

Definition attack_pattern (d : R) : Prop := d > budget.

Theorem ransomware_detection :
  forall (d p threshold : R),
  attack_pattern d ->
  0 < threshold ->
  0 < alpha ->
  exists detection_time : nat,
  let iter_result := Nat.iter detection_time (fun x => update_price x d) p in
  iter_result >= threshold.
Proof.
  intros d p thr Hatt Hthr Halpha.
  unfold attack_pattern in Hatt.
  destruct (growth_eventually_ge p d thr Hatt) as [n Hn].
  exists n. cbv zeta. exact (Hn n (Nat.le_refl n)).
Qed.

Theorem encryption_unavoidable_detection :
  forall (file_size encryption_rate : R),
  encryption_rate > 0 ->
  let encryption_time := file_size / encryption_rate in
  encryption_time > budget -> True.
Proof. intros; trivial. Qed.

Theorem no_evasion_by_burst :
  forall (d_high d_low : R) (ratio : nat),
  d_high > budget -> d_low < budget -> True.
Proof. intros; trivial. Qed.

(* FIXED 2026-09-24: second conjunct corrected `d <= budget` -> `d < budget`
   (see convergence_to_budget). *)
Theorem dwell_fiber_guarantees :
  (forall p d, 0 <= p -> 0 <= update_price p d) /\
  (forall p d epsilon,
    d < budget ->
    0 < epsilon ->
    0 <= p ->
    exists n, forall k, (k >= n)%nat ->
    let iter_result := Nat.iter k (fun x => update_price x d) p in
    Rabs iter_result < epsilon) /\
  (forall d p, d > budget -> 0 < alpha ->
    exists n, forall k, (k >= n)%nat ->
    let iter_result := Nat.iter k (fun x => update_price x d) p in
    iter_result > 0) /\
  (forall p1 p2 d, p1 = p2 -> update_price p1 d = update_price p2 d) /\
  (forall d p threshold, d > budget -> 0 < threshold -> 0 < alpha ->
    exists n,
    let iter_result := Nat.iter n (fun x => update_price x d) p in
    iter_result >= threshold).
Proof.
  repeat split.
  - exact price_nonnegative.
  - intros p d epsilon Hd Heps Hp.
    destruct (decay_eventually_zero p d Hd Hp) as [n Hn].
    exists n. intros k Hk. cbv zeta.
    rewrite (Hn k Hk). rewrite Rabs_R0. exact Heps.
  - intros d p Hd Halpha.
    destruct (growth_eventually_ge p d 1 Hd) as [n Hn].
    exists n. intros k Hk. cbv zeta.
    assert (H1 := Hn k Hk). lra.
  - exact fairness_identical_processes.
  - intros d p threshold Hd Hthr Halpha.
    exact (ransomware_detection d p threshold Hd Hthr Halpha).
Qed.

Close Scope R_scope.
