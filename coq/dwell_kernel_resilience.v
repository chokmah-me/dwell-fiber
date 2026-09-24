(* Dwell-Fiber Kernel-Userspace Resilience Model
   Formalizes bounded event loss from eBPF layer and proves ADMM stability *)

Require Import Reals.
From Coq Require Import ZArith.
Require Import Lia.
Require Import Nat.
Require Import Lra.
Require Import List.
From Coq Require Import Arith.Arith.
Require Import RIneq.
From Coq Require Import Psatz.
Import ListNotations.

Open Scope R_scope.

(* ========================================================================== *)
(* SECTION 1: Event Stream Model *)
(* ========================================================================== *)

Inductive event_type : Type :=
  | FileRead : nat -> event_type
  | FileWrite : nat -> event_type
  | FileOpen : nat -> event_type
  | FileClose : nat -> event_type.

Record event : Type := mk_event {
  ev_type : event_type;
  ev_timestamp : R;
  ev_dwell : R;
  ev_process_id : nat
}.

Definition event_stream := list event.

(* Extract total dwell from a stream *)
Fixpoint total_dwell (stream : event_stream) : R :=
  match stream with
  | [] => 0
  | e :: rest => ev_dwell e + total_dwell rest
  end.

(* ========================================================================== *)
(* SECTION 2: Bounded Event Loss Model *)
(* ========================================================================== *)

Inductive loss_pattern : Type :=
  | Keep : loss_pattern  (* Event is kept *)
  | Drop : loss_pattern. (* Event is dropped *)

Parameter delta : R.  (* Maximum loss rate: 0 <= delta < 1 *)
Parameter max_burst_loss : nat.  (* Maximum consecutive drops *)

Axiom delta_pos : 0 <= delta.
Axiom delta_lt_1 : delta < 1.
Axiom max_burst_positive : (max_burst_loss > 0)%nat.

(* Apply loss pattern to event stream *)
Fixpoint apply_loss (stream : event_stream) (pattern : list loss_pattern) : event_stream :=
  match stream, pattern with
  | [], _ => []
  | _, [] => stream  (* No more pattern = keep remaining events *)
  | e :: rest, p :: pat_rest =>
      match p with
      | Keep => e :: apply_loss rest pat_rest
      | Drop => apply_loss rest pat_rest
      end
  end.

(* Count consecutive drops in a pattern prefix *)
Fixpoint count_consecutive_drops (pattern : list loss_pattern) : nat :=
  match pattern with
  | [] => 0
  | Drop :: rest => 1 + count_consecutive_drops rest
  | Keep :: _ => 0
  end.

(* Check if a loss pattern is valid for a given stream *)
Definition valid_loss_pattern (original_stream : event_stream)
                               (pattern : list loss_pattern) : Prop :=
  let kept_events := apply_loss original_stream pattern in
  let total_original := length original_stream in
  let total_kept := length kept_events in
  let loss_count := (total_original - total_kept)%nat in

  (* Constraint 1: Loss rate <= delta *)
  (INR loss_count <= delta * INR total_original) /\

  (* Constraint 2: No burst loss exceeds max_burst_loss *)
  (forall (n : nat) (subpattern : list loss_pattern),
     subpattern = firstn n pattern ->
     (count_consecutive_drops subpattern <= max_burst_loss)%nat).

(* ========================================================================== *)
(* SECTION 3: ADMM Price Update with Streams *)
(* ========================================================================== *)

(* Import parameters from dwell_stable.v *)
Parameter alpha : R.
Parameter budget : R.
Axiom alpha_pos : 0 < alpha.
Axiom alpha_lt_2 : alpha < 2.
Axiom budget_is_five : budget = 5.

Definition price := R.
Definition dwell := R.

(* Original update_price from dwell_stable.v *)
Definition update_price (p : price) (d : dwell) : price :=
  Rmax 0 (p + alpha * (d - budget)).

(* Update price based on entire event stream *)
Definition update_price_from_stream (p : price) (stream : event_stream) : price :=
  let total_d := total_dwell stream in
  update_price p total_d.

(* Maximum dwell per event - needed for the dwell arithmetic helpers *)
Parameter max_dwell_per_event : R.
Axiom max_dwell_positive : max_dwell_per_event > 0.
Axiom max_dwell_bound :
  forall (stream : event_stream) (e : event),
  In e stream -> 0 <= ev_dwell e <= max_dwell_per_event.

(* ========================================================================== *)
(* SECTION 3B: Helper lemmas - dwell arithmetic under loss                     *)
(* ========================================================================== *)

Lemma total_dwell_nonneg : forall (stream : event_stream),
  0 <= total_dwell stream.
Proof.
  intros stream. induction stream as [|e rest IH].
  - simpl. apply Rle_refl.
  - simpl.
    assert (Hin : In e (e :: rest)) by apply in_eq.
    destruct (max_dwell_bound (e :: rest) e Hin) as [H _].
    lra.
Qed.

Lemma total_dwell_apply_loss_le : forall (stream : event_stream) (pattern : list loss_pattern),
  total_dwell (apply_loss stream pattern) <= total_dwell stream.
Proof.
  intros stream pattern.
  generalize dependent pattern.
  induction stream as [|e rest IH]; intros pattern.
  - simpl. apply Rle_refl.
  - destruct pattern as [|q pat].
    + simpl. apply Rle_refl.
    + destruct q.
      * simpl. apply Rplus_le_compat_l. apply IH.
      * simpl.
        assert (H := IH pat).
        assert (Hin : In e (e :: rest)) by apply in_eq.
        destruct (max_dwell_bound (e :: rest) e Hin) as [He _].
        assert (Hnn : 0 <= total_dwell (apply_loss rest pat)) by apply total_dwell_nonneg.
        lra.
Qed.

Lemma length_apply_loss_le : forall (stream : event_stream) (pattern : list loss_pattern),
  (length (apply_loss stream pattern) <= length stream)%nat.
Proof.
  intros stream pattern.
  generalize dependent pattern.
  induction stream as [|e rest IH]; intros pattern.
  - simpl. lia.
  - destruct pattern as [|q pat].
    + simpl. lia.
    + destruct q; simpl.
      * assert (H := IH pat). lia.
      * assert (H := IH pat). lia.
Qed.

Lemma in_apply_loss : forall (stream : event_stream) (pattern : list loss_pattern) (e : event),
  In e (apply_loss stream pattern) -> In e stream.
Proof.
  intros stream pattern.
  generalize dependent pattern.
  induction stream as [|x rest IH]; intros pattern e Hin.
  - simpl in Hin. destruct Hin.
  - destruct pattern as [|q pat].
    + simpl in Hin. exact Hin.
    + destruct q; simpl in Hin.
      * apply in_inv in Hin. destruct Hin as [Heq | Hin'].
        { rewrite Heq. apply in_eq. }
        { apply in_cons. apply (IH pat e Hin'). }
      * apply in_cons. apply (IH pat e Hin).
Qed.

(* If no events were dropped (lengths agree), the loss was the identity. *)
Lemma apply_loss_eq_of_length : forall (stream : event_stream) (pattern : list loss_pattern),
  length (apply_loss stream pattern) = length stream ->
  apply_loss stream pattern = stream.
Proof.
  intros stream pattern.
  generalize dependent pattern.
  induction stream as [|x rest IH]; intros pattern Hlen.
  - simpl. reflexivity.
  - destruct pattern as [|q pat].
    + simpl. reflexivity.
    + destruct q.
      * simpl in Hlen. simpl.
        assert (Hlen' : length (apply_loss rest pat) = length rest) by lia.
        rewrite (IH pat Hlen'). reflexivity.
      * simpl in Hlen.
        assert (Hle := length_apply_loss_le rest pat).
        lia.
Qed.

(* Uniform per-event dwell gives a closed form for total dwell. *)
Lemma total_dwell_uniform : forall (stream : event_stream) (c : R),
  (forall e, In e stream -> ev_dwell e = c) ->
  total_dwell stream = INR (length stream) * c.
Proof.
  intros stream c Huni.
  induction stream as [|e rest IH].
  - change (total_dwell []) with 0.
    change (length []) with 0%nat.
    assert (H0 : INR 0 = 0) by reflexivity.
    rewrite H0. rewrite Rmult_0_l. reflexivity.
  - change (total_dwell (e :: rest)) with (ev_dwell e + total_dwell rest).
    change (length (e :: rest)) with (S (length rest)).
    assert (He : ev_dwell e = c).
    { apply Huni. apply in_eq. }
    assert (IH' : total_dwell rest = INR (length rest) * c).
    { apply IH. intros e' Hin. apply Huni. apply in_cons. exact Hin. }
    rewrite He, IH', S_INR. ring.
Qed.

(* Event-count form of the loss bound: at least (1-delta) of events survive. *)
Lemma kept_count_bound : forall (original_stream : event_stream) (pattern : list loss_pattern),
  valid_loss_pattern original_stream pattern ->
  (1 - delta) * INR (length original_stream) <= INR (length (apply_loss original_stream pattern)).
Proof.
  intros original_stream pattern Hvalid.
  unfold valid_loss_pattern in Hvalid.
  cbv zeta in Hvalid.
  destruct Hvalid as [Hrate _].
  assert (Hle : (length (apply_loss original_stream pattern) <= length original_stream)%nat)
    by apply length_apply_loss_le.
  assert (Hadd : (length original_stream - length (apply_loss original_stream pattern)
                  + length (apply_loss original_stream pattern) = length original_stream)%nat) by lia.
  assert (Heq : INR (length original_stream - length (apply_loss original_stream pattern))
                + INR (length (apply_loss original_stream pattern))
                = INR (length original_stream)).
  { rewrite <- plus_INR. rewrite Hadd. reflexivity. }
  replace ((1 - delta) * INR (length original_stream))
    with (INR (length original_stream) - delta * INR (length original_stream)) by ring.
  lra.
Qed.

(* The price map d |-> Rmax 0 (p + alpha*(d-budget)) is alpha-Lipschitz. *)
Lemma Rmax0_lipschitz : forall x y : R,
  Rabs (Rmax 0 x - Rmax 0 y) <= Rabs (x - y).
Proof.
  intros x y.
  destruct (Rle_dec x 0) as [Hx | Hnx];
  destruct (Rle_dec y 0) as [Hy | Hny].
  - rewrite (Rmax_left 0 x Hx), (Rmax_left 0 y Hy), Rminus_0_r, Rabs_R0.
    apply Rabs_pos.
  - assert (Hy' : 0 < y) by exact (Rnot_le_lt _ _ Hny).
    rewrite (Rmax_left 0 x Hx), (Rmax_right 0 y (Rlt_le _ _ Hy')).
    assert (E1 : Rabs (0 - y) = y).
    { replace (0 - y) with (-y) by ring. rewrite Rabs_Ropp.
      rewrite (Rabs_pos_eq y (Rlt_le _ _ Hy')). reflexivity. }
    assert (E2 : Rabs (x - y) = y - x).
    { replace (x - y) with (-(y - x)) by ring. rewrite Rabs_Ropp.
      rewrite (Rabs_pos_eq (y - x)). reflexivity. lra. }
    rewrite E1, E2. lra.
  - assert (Hx' : 0 < x) by exact (Rnot_le_lt _ _ Hnx).
    rewrite (Rmax_right 0 x (Rlt_le _ _ Hx')), (Rmax_left 0 y Hy).
    assert (E1 : Rabs (x - 0) = x).
    { replace (x - 0) with x by ring.
      rewrite (Rabs_pos_eq x (Rlt_le _ _ Hx')). reflexivity. }
    assert (E2 : Rabs (x - y) = x - y).
    { rewrite (Rabs_pos_eq (x - y)). reflexivity. lra. }
    rewrite E1, E2. lra.
  - assert (Hx' : 0 < x) by exact (Rnot_le_lt _ _ Hnx).
    assert (Hy' : 0 < y) by exact (Rnot_le_lt _ _ Hny).
    rewrite (Rmax_right 0 x (Rlt_le _ _ Hx')), (Rmax_right 0 y (Rlt_le _ _ Hy')).
    apply Rle_refl.
Qed.

(* ========================================================================== *)
(* SECTION 4: Critical Lemma 1 - Bounded Loss Preserves Dwell Bound *)
(* ========================================================================== *)

(* RESTATED 2026-09-24: the original claim -- dwell preserved proportionally
   to event counts with no hypothesis on per-event dwell -- is FALSE.
   Counterexample: one event with dwell 100 and 99 events with dwell 0.01;
   dropping 50% of the events including the large one leaves dwell 0.50
   against a claimed lower bound of ~50.49. The honest version assumes
   uniform per-event dwell; the pure count-level bound is kept_count_bound. *)
Lemma bounded_loss_preserves_dwell_bound :
  forall (original_stream : event_stream)
         (pattern : list loss_pattern)
         (true_total_dwell : R)
         (d0 : R),
  0 <= d0 ->
  (forall e, In e original_stream -> ev_dwell e = d0) ->
  valid_loss_pattern original_stream pattern ->
  total_dwell original_stream = true_total_dwell ->
  (1 - delta) * true_total_dwell <= total_dwell (apply_loss original_stream pattern).
Proof.
  intros original_stream pattern true_total_dwell d0 Hd0 Huni Hvalid Htotal.
  assert (Huni_kept : forall e, In e (apply_loss original_stream pattern) -> ev_dwell e = d0).
  { intros e Hin. apply Huni. apply (in_apply_loss _ _ _ Hin). }
  rewrite (total_dwell_uniform _ _ Huni_kept).
  rewrite <- Htotal.
  rewrite (total_dwell_uniform _ _ Huni).
  assert (Hcount := kept_count_bound original_stream pattern Hvalid).
  assert (Hmul : (1 - delta) * INR (length original_stream) * d0
                <= INR (length (apply_loss original_stream pattern)) * d0).
  { apply Rmult_le_compat_r. exact Hd0. exact Hcount. }
  replace ((1 - delta) * (INR (length original_stream) * d0))
    with ((1 - delta) * INR (length original_stream) * d0) by ring.
  exact Hmul.
Qed.


(* ========================================================================== *)
(* SECTION 5: Critical Lemma 2 - Price Update Monotonicity *)
(* ========================================================================== *)

Lemma update_price_monotonic :
  forall (p : price) (d1 d2 : dwell),
  0 <= p ->
  0 <= d1 -> d1 <= d2 ->
  update_price p d1 <= update_price p d2.
Proof.
  intros p d1 d2 Hp Hd1 Hd2.
  unfold update_price.
  destruct (Rle_dec (p + alpha * (d1 - budget)) 0) as [H1|H1].
  - (* d1 case: max returns 0 *)
    rewrite Rmax_left by assumption.
    apply Rmax_l.
  - (* d1 case: max returns p + alpha*(d1-budget) *)
    rewrite Rmax_right; [|lra].
    destruct (Rle_dec (p + alpha * (d2 - budget)) 0) as [H2|H2].
    + (* Edge case: This is actually impossible - derive contradiction *)
      (* From H1: ~(p + alpha * (d1 - budget) <= 0), i.e., 0 < p + alpha * (d1 - budget) *)
      (* From H2: p + alpha * (d2 - budget) <= 0 *)
      (* From Hd2: d1 <= d2, so alpha*(d1-budget) <= alpha*(d2-budget) since alpha > 0 *)
      (* This gives p + alpha*(d1-budget) <= p + alpha*(d2-budget) <= 0, contradicting H1 *)
      exfalso.
      assert (d1 - budget <= d2 - budget) by (apply Rplus_le_compat_r; exact Hd2).
      assert (alpha * (d1 - budget) <= alpha * (d2 - budget)) by (apply Rmult_le_compat_l; [apply Rlt_le; exact alpha_pos | exact H]).
      assert (p + alpha * (d1 - budget) <= p + alpha * (d2 - budget)) by (apply Rplus_le_compat_l; exact H0).
      assert (p + alpha * (d1 - budget) <= 0) by (apply Rle_trans with (p + alpha * (d2 - budget)); assumption).
      exact (H1 H4).
    + (* Both max return positive values *)
      rewrite Rmax_right; [|lra].
      apply Rplus_le_compat_l.
      apply Rmult_le_compat_l.
      * apply Rlt_le; exact alpha_pos.
      * assert (d1 - budget <= d2 - budget) as Hdiff.
        { apply Rplus_le_compat_r. exact Hd2. }
        exact Hdiff.
Qed.

Lemma price_update_monotonic_dwell :
  forall (p : price) (stream1 stream2 : event_stream),
  0 <= p ->
  total_dwell stream1 <= total_dwell stream2 ->
  update_price_from_stream p stream1 <= update_price_from_stream p stream2.
Proof.
  intros p stream1 stream2 Hp Hdwell.
  unfold update_price_from_stream.
  apply update_price_monotonic.
  - exact Hp.
  - apply total_dwell_nonneg.
  - exact Hdwell.
Qed.

(* ========================================================================== *)
(* SECTION 6: Critical Lemma 3 - Bounded Price Under Loss *)
(* ========================================================================== *)

Lemma bounded_price_under_loss :
  forall (initial_price : price)
         (original_stream : event_stream)
         (pattern : list loss_pattern),
  0 <= initial_price ->
  valid_loss_pattern original_stream pattern ->
  let final_price := update_price_from_stream initial_price
                                            (apply_loss original_stream pattern) in
  0 <= final_price <= initial_price + alpha * total_dwell original_stream.
Proof.
  intros initial_price original_stream pattern Hprice Hvalid.
  cbv zeta.
  split.
  - unfold update_price_from_stream, update_price. cbv zeta. apply Rmax_l.
  - unfold update_price_from_stream, update_price. cbv zeta.
    assert (Hle : total_dwell (apply_loss original_stream pattern) <= total_dwell original_stream)
      by apply total_dwell_apply_loss_le.
    assert (Hnn2 : 0 <= total_dwell original_stream) by apply total_dwell_nonneg.
    assert (Hb : 0 < budget) by (rewrite budget_is_five; lra).
    destruct (Rle_dec (initial_price + alpha * (total_dwell (apply_loss original_stream pattern) - budget)) 0)
      as [Hdec | Hndec].
    + rewrite (Rmax_left 0 _ Hdec).
      assert (Hpos : 0 <= alpha * total_dwell original_stream).
      { apply Rmult_le_pos. apply Rlt_le. apply alpha_pos. exact Hnn2. }
      lra.
    + assert (Hlt : 0 < initial_price + alpha * (total_dwell (apply_loss original_stream pattern) - budget))
        by exact (Rnot_le_lt _ _ Hndec).
      rewrite (Rmax_right 0 _ (Rlt_le _ _ Hlt)).
      assert (H2 : total_dwell (apply_loss original_stream pattern) - budget
                   <= total_dwell original_stream) by lra.
      assert (H1 : alpha * (total_dwell (apply_loss original_stream pattern) - budget)
                   <= alpha * total_dwell original_stream).
      { apply Rmult_le_compat_l. apply Rlt_le. apply alpha_pos. exact H2. }
      lra.
Qed.

(* ========================================================================== *)
(* SECTION 7: Bridge to dwell_stable.v *)
(* ========================================================================== *)

(* This lemma connects stream-based updates to iteration-based model *)
Lemma lossy_stream_stability_bridge :
  forall (p : price) (stream : event_stream) (pattern : list loss_pattern),
  valid_loss_pattern stream pattern ->
  let effective_dwell := total_dwell (apply_loss stream pattern) in
  update_price_from_stream p (apply_loss stream pattern) =
  update_price p effective_dwell.
Proof.
  intros p stream pattern Hvalid.
  unfold update_price_from_stream.
  reflexivity.
Qed.

(* ========================================================================== *)
(* SECTION 8: Main Resilience Theorem *)
(* ========================================================================== *)

(* RESTATED 2026-09-24: the original claim -- deviation of the lossy price
   from `budget` bounded by an arbitrary epsilon for any valid pattern --
   is FALSE. Counterexample: alpha = 1, budget = 5, initial_price = 5,
   empty stream: the update drives the price to Rmax 0 (5 + 1*(0-5)) = 0,
   so |final_price - budget| = 5 no matter how small epsilon is.
   The honest resilience statement is Lipschitz in the lost dwell:
   event loss perturbs the price by at most alpha times the dwell lost,
   relative to the lossless trajectory. *)
Theorem admm_resilience_to_event_loss :
  forall (initial_price : price)
         (original_stream : event_stream)
         (pattern : list loss_pattern),
  0 <= initial_price ->
  valid_loss_pattern original_stream pattern ->
  let ideal_price := update_price_from_stream initial_price original_stream in
  let actual_price := update_price_from_stream initial_price (apply_loss original_stream pattern) in
  Rabs (actual_price - ideal_price)
    <= alpha * (total_dwell original_stream - total_dwell (apply_loss original_stream pattern)).
Proof.
  intros initial_price original_stream pattern Hprice Hvalid.
  cbv zeta.
  unfold update_price_from_stream, update_price. cbv zeta.
  assert (Hle : total_dwell (apply_loss original_stream pattern) <= total_dwell original_stream)
    by apply total_dwell_apply_loss_le.
  assert (Hnn : 0 <= total_dwell original_stream - total_dwell (apply_loss original_stream pattern)) by lra.
  assert (Hlip := Rmax0_lipschitz
    (initial_price + alpha * (total_dwell (apply_loss original_stream pattern) - budget))
    (initial_price + alpha * (total_dwell original_stream - budget))).
  assert (Hdiff : (initial_price + alpha * (total_dwell (apply_loss original_stream pattern) - budget))
                 - (initial_price + alpha * (total_dwell original_stream - budget))
                 = alpha * (total_dwell (apply_loss original_stream pattern) - total_dwell original_stream)).
  { ring. }
  rewrite Hdiff in Hlip.
  eapply Rle_trans.
  - exact Hlip.
  - replace (alpha * (total_dwell (apply_loss original_stream pattern) - total_dwell original_stream))
      with (-(alpha * (total_dwell original_stream - total_dwell (apply_loss original_stream pattern)))) by ring.
    rewrite Rabs_Ropp.
    assert (Hpos : 0 <= alpha * (total_dwell original_stream - total_dwell (apply_loss original_stream pattern))).
    { apply Rmult_le_pos. apply Rlt_le. apply alpha_pos. exact Hnn. }
    rewrite (Rabs_pos_eq _ Hpos).
    apply Rle_refl.
Qed.

Close Scope R_scope.
