(* Dwell-Fiber Extended Proofs *)
Require Import Reals.
From Coq Require Import ZArith.
From Coq Require Import Lra.
Require Import Lia.
Require Import Nat.
Require Import List.
Open Scope R_scope.

(* Ceiling: up r returns smallest integer >= r *)
Definition nat_ceil (r : R) : nat :=
  Z.to_nat (up r).

Parameter alpha budget throttle_threshold kill_threshold : R.
Axiom alpha_range : 0 < alpha /\ alpha < 2.
Axiom budget_positive : 0 < budget.
Axiom throttle_positive : 0 < throttle_threshold.
Axiom kill_positive : 0 < kill_threshold.
Axiom threshold_order : throttle_threshold < kill_threshold.

Record process_state := {
  pid : nat;
  current_price : R;
  current_dwell : R;
  throttled : bool;
  killed : bool;
  enforcement_count : nat;
}.

Definition update_price (p d : R) : R :=
  Rmax 0 (p + alpha * (d - budget)).

Theorem price_nonnegative :
  forall (p d : R), 0 <= p -> 0 <= update_price p d.
Proof. intros p d Hp; unfold update_price; apply Rmax_l. Qed.

(* ================================================================== *)
(* Helper lemmas: closed forms for the price iteration (same           *)
(* mathematics as dwell_stable.v, proved locally to keep this file    *)
(* self-contained; this file's alpha axiom is alpha_range).           *)
(* ================================================================== *)

Lemma div_mul_cancel : forall (r c : R),
  c <> 0 -> (r / c) * c = r.
Proof.
  intros r c Hc0.
  unfold Rdiv. rewrite Rmult_assoc. rewrite (Rmult_comm (/ c) c).
  rewrite (Rinv_r c Hc0). rewrite Rmult_1_r. reflexivity.
Qed.

Lemma archimed_nat_mul : forall (r c : R),
  0 < c -> exists n : nat, r <= INR n * c.
Proof.
  intros r c Hc.
  assert (Hc0 : c <> 0) by lra.
  destruct (archimed (r / c)) as [Hgt _].
  revert Hgt.
  destruct (up (r / c)) as [|p|p] eqn:Hz.
  - intros Hgt.
    exists 0%nat.
    assert (H0 : IZR Z0 = 0) by reflexivity.
    assert (Hr : r < 0).
    { assert (H2 : (r / c) * c < 0 * c).
      { apply Rmult_lt_compat_r. exact Hc. lra. }
      rewrite Rmult_0_l in H2. rewrite (div_mul_cancel r c Hc0) in H2. exact H2. }
    assert (HINR0 : INR 0 = 0) by reflexivity.
    rewrite HINR0. rewrite Rmult_0_l. lra.
  - intros Hgt.
    exists (Z.to_nat (Zpos p)).
    assert (Hz0 : (0 <= Zpos p)%Z) by lia.
    assert (Heq : INR (Z.to_nat (Zpos p)) = IZR (Zpos p)).
    { rewrite INR_IZR_INZ. rewrite (Z2Nat.id (Zpos p) Hz0). reflexivity. }
    rewrite Heq.
    assert (Hlt : (r / c) * c < IZR (Zpos p) * c).
    { apply Rmult_lt_compat_r. exact Hc. exact Hgt. }
    rewrite (div_mul_cancel r c Hc0) in Hlt.
    lra.
  - intros Hgt.
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

Lemma decay_eventually_zero : forall (p d : R),
  d < budget -> 0 <= p ->
  exists n : nat, forall k : nat, (k >= n)%nat ->
    Nat.iter k (fun x => update_price x d) p = 0.
Proof.
  intros p d Hd Hp.
  destruct alpha_range as [Hapos _].
  assert (Hc : 0 < alpha * (budget - d)).
  { apply Rmult_lt_0_compat. exact Hapos. lra. }
  destruct (archimed_nat_mul p (alpha * (budget - d)) Hc) as [n Hn].
  exists n. intros k Hk.
  rewrite (iter_decay_closed p d k Hp Hc).
  apply Rmax_left.
  assert (Hkn : INR n <= INR k) by (apply le_INR; exact Hk).
  assert (H1 : INR n * (alpha * (budget - d)) <= INR k * (alpha * (budget - d))).
  { apply Rmult_le_compat_r. apply Rlt_le. exact Hc. exact Hkn. }
  lra.
Qed.

Lemma growth_eventually_ge : forall (p d thr : R),
  d > budget -> 0 < thr ->
  exists n : nat, forall k : nat, (k >= n)%nat ->
    Nat.iter k (fun x => update_price x d) p >= thr.
Proof.
  intros p d thr Hd Hthr.
  destruct alpha_range as [Hapos _].
  assert (Hc : 0 < alpha * (d - budget)).
  { apply Rmult_lt_0_compat. exact Hapos. lra. }
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

(* ================================================================== *)
(* Deterministic enforcement policy: flags are a function of price.    *)
(* ================================================================== *)

Definition enforce_throttled (p : R) : bool :=
  if Rlt_dec throttle_threshold p then
    if Rlt_dec kill_threshold p then false else true
  else false.

Definition enforce_killed (p : R) : bool :=
  if Rlt_dec kill_threshold p then true else false.

Definition terminal_state (s : process_state) : Prop :=
  (s.(current_price) <= throttle_threshold /\ s.(throttled)=false /\ s.(killed)=false) \/
  (throttle_threshold < s.(current_price) /\ s.(current_price) <= kill_threshold /\ s.(throttled)=true) \/
  (s.(current_price) > kill_threshold /\ s.(killed)=true).

(* A price at or below the throttle threshold is terminal (clean) under
   the policy. *)
Lemma policy_terminal : forall (pid : nat) (p d : R) (ec : nat),
  p <= throttle_threshold ->
  terminal_state {| pid := pid; current_price := p; current_dwell := d;
                    throttled := enforce_throttled p; killed := enforce_killed p;
                    enforcement_count := ec |}.
Proof.
  intros pid p d ec Hle.
  unfold terminal_state. simpl.
  unfold enforce_throttled, enforce_killed.
  destruct (Rlt_dec throttle_threshold p) as [Ht|Hnt];
    destruct (Rlt_dec kill_threshold p) as [Hk|Hnk];
    simpl.
  - exfalso. lra.
  - exfalso. lra.
  - exfalso. pose proof threshold_order as Hord. lra.
  - left. split.
    + apply Rnot_lt_le. exact Hnt.
    + split; reflexivity.
Qed.

(* RESTATED 2026-09-24: two fixes. (1) d <= budget is FALSE at d = budget
   (the price is a fixed point there); the decay argument needs d < budget.
   (2) The original flag conjuncts (s.(throttled)=false, s.(killed)=false)
   are unprovable -- the premises say nothing about the flags. The honest
   version concludes the process reaches a policy-terminal clean state. *)
Theorem liveness_normal_operation :
  forall (s : process_state),
  s.(current_dwell) < budget ->
  0 <= s.(current_price) ->
  exists n : nat,
  forall (k : nat),
  (k >= n)%nat ->
  let updated_price := Nat.iter k (fun p => update_price p s.(current_dwell)) s.(current_price) in
  updated_price <= throttle_threshold /\
  terminal_state {| pid := s.(pid); current_price := updated_price; current_dwell := s.(current_dwell);
                    throttled := enforce_throttled updated_price;
                    killed := enforce_killed updated_price;
                    enforcement_count := s.(enforcement_count) |}.
Proof.
  intros s Hd Hp.
  destruct (decay_eventually_zero s.(current_price) s.(current_dwell) Hd Hp) as [n Hn].
  exists n. intros k Hk.
  cbv zeta.
  assert (Hz : Nat.iter k (fun p => update_price p s.(current_dwell)) s.(current_price) = 0)
    by exact (Hn k Hk).
  assert (Ht : 0 <= throttle_threshold) by (pose proof throttle_positive as Htp; lra).
  split.
  - rewrite Hz. exact Ht.
  - rewrite Hz. apply policy_terminal. exact Ht.
Qed.

Theorem liveness_under_attack :
  forall (s : process_state),
  s.(current_dwell) > budget ->
  0 <= s.(current_price) ->
  (exists n : nat,
     (forall (k : nat), (k >= n)%nat ->
        let updated_price := Nat.iter k (fun p => update_price p s.(current_dwell)) s.(current_price) in
        updated_price >= throttle_threshold) \/
     (forall (k : nat), (k >= n)%nat ->
        let updated_price := Nat.iter k (fun p => update_price p s.(current_dwell)) s.(current_price) in
        updated_price >= kill_threshold)).
Proof.
  intros s Hd Hp.
  destruct (growth_eventually_ge s.(current_price) s.(current_dwell) kill_threshold Hd kill_positive) as [n Hn].
  exists n. right. intros k Hk.
  cbv zeta. exact (Hn k Hk).
Qed.

(* RESTATED 2026-09-24: the original is FALSE at d = budget, where the price
   is a fixed point of the update and can sit strictly between the thresholds
   forever (e.g. price constantly (throttle+kill)/2). The honest version
   excludes the fixed-point case. *)
Theorem no_livelock :
  forall (s : process_state),
  s.(current_dwell) <> budget ->
  ~ (exists inf_loop : nat -> R,
      (forall (k : nat),
        inf_loop (k + 1)%nat = update_price (inf_loop k) s.(current_dwell) /\
        inf_loop 0%nat = s.(current_price) /\
        (forall n : nat, throttle_threshold < inf_loop n /\ inf_loop n < kill_threshold))).
Proof.
  intros s Hneq [inf_loop H].
  pose proof throttle_positive as Htp.
  (* the alleged loop is the deterministic price iteration *)
  assert (Hiter : forall k : nat,
      inf_loop k = Nat.iter k (fun p => update_price p s.(current_dwell)) s.(current_price)).
  { intro k. induction k as [|k IH].
    - destruct (H 0%nat) as [_ [Hinit _]]. simpl. exact Hinit.
    - destruct (H k) as [Hstep _].
      rewrite Nat.iter_succ. cbv beta.
      replace (S k) with (k + 1)%nat by lia.
      rewrite Hstep. rewrite IH. reflexivity. }
  destruct (H 0%nat) as [_ [Hinit Hrange]].
  destruct (Hrange 0%nat) as [Hthr0 _].
  assert (Hnn : 0 <= s.(current_price)) by lra.
  destruct (total_order_T s.(current_dwell) budget) as [[Hlt | Heq] | Hgt].
  - (* d < budget: the iteration hits exactly 0, below the throttle floor *)
    destruct (decay_eventually_zero s.(current_price) s.(current_dwell) Hlt Hnn) as [n Hn].
    destruct (Hrange n) as [Hthr _].
    rewrite Hiter in Hthr.
    rewrite (Hn n (Nat.le_refl n)) in Hthr.
    lra.
  - exact (Hneq Heq).
  - (* d > budget: the iteration reaches kill_threshold, above the ceiling *)
    destruct (growth_eventually_ge s.(current_price) s.(current_dwell) kill_threshold Hgt kill_positive) as [n Hn].
    destruct (Hrange n) as [_ Hkill].
    rewrite Hiter in Hkill.
    assert (Hge := Hn n (Nat.le_refl n)).
    lra.
Qed.

Definition fair_pricing (processes : list process_state) : Prop :=
  forall (p1 p2 : process_state),
    In p1 processes -> In p2 processes ->
    p1.(current_dwell) = p2.(current_dwell) ->
    p1.(current_price) = p2.(current_price) ->
    (p1.(throttled)=true <-> p2.(throttled)=true) /\
    (p1.(killed)=true <-> p2.(killed)=true).

(* RESTATED 2026-09-24: the original premises say nothing about the
   throttled/killed flags, so its conclusion does not follow (two processes
   can share dwell and price yet carry different flags). The honest version
   fixes a deterministic enforcement policy -- flags are a function of price
   -- and proves the policy treats equal processes equally. *)
Theorem fair_pricing_theorem :
  forall (processes : list process_state),
  (forall p : process_state,
     In p processes ->
     p.(throttled) = enforce_throttled p.(current_price) /\
     p.(killed) = enforce_killed p.(current_price)) ->
  fair_pricing processes.
Proof.
  intros ps Hpol. unfold fair_pricing.
  intros p1 p2 Hin1 Hin2 _ Hprice.
  destruct (Hpol p1 Hin1) as [Ht1 Hk1].
  destruct (Hpol p2 Hin2) as [Ht2 Hk2].
  split.
  - rewrite Ht1, Ht2, Hprice. apply iff_refl.
  - rewrite Hk1, Hk2, Hprice. apply iff_refl.
Qed.

Theorem attack_detection_bounded :
  forall (s : process_state),
  s.(current_dwell) > budget ->
  0 <= s.(current_price) ->
  exists max_iterations : nat,
  forall (k : nat),
  (k >= max_iterations)%nat ->
  let updated_price := Nat.iter k (fun p => update_price p s.(current_dwell)) s.(current_price) in
  updated_price > throttle_threshold.
Proof.
  intros s Hd Hp.
  pose proof throttle_positive as Htp.
  assert (Hpos : 0 < throttle_threshold + 1) by lra.
  destruct (growth_eventually_ge s.(current_price) s.(current_dwell) (throttle_threshold + 1) Hd Hpos)
    as [n Hn].
  exists n. intros k Hk.
  cbv zeta.
  assert (H := Hn k Hk).
  lra.
Qed.

Theorem enforcement_terminates :
  forall (s : process_state),
  s.(current_dwell) > budget ->
  0 <= s.(current_price) ->
  exists termination_time : nat,
  let final_price := Nat.iter termination_time (fun p => update_price p s.(current_dwell)) s.(current_price) in
  final_price >= kill_threshold \/ final_price >= throttle_threshold.
Proof.
  intros s Hd Hp.
  destruct (growth_eventually_ge s.(current_price) s.(current_dwell) kill_threshold Hd kill_positive) as [n Hn].
  exists n. cbv zeta.
  left. exact (Hn n (Nat.le_refl n)).
Qed.

Close Scope R_scope.

(* RESTATED 2026-09-24: the original (pid > 0 -> pid < 65536) is unprovable --
   no PID upper bound exists in the model. The honest version takes the
   OS-enforced bound as an explicit premise. *)
Theorem process_safety_nonempty :
  forall (s : process_state),
  (s.(pid) > 0)%nat ->
  (s.(pid) < 65536)%nat ->
  (0 < s.(pid) < 65536)%nat.
Proof.
  intros s Hpos Hbound. lia.
Qed.
