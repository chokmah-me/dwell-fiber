(* Dwell-Fiber Resilience Proof Test Suite
   Tests for kernel-userspace resilience model.

   REWRITTEN 2026-09-24: updated to the restated dwell_kernel_resilience.v API.
   - Import fixed: DwellKernelResilience -> DwellFiber.dwell_kernel_resilience
     (the Makefile uses -R . DwellFiber, so the unqualified name never resolved).
   - test_lemma1_keep_all / test_lemma1_empty: bounded_loss_preserves_dwell_bound
     now requires an explicit uniform-dwell premise (the old statement was false;
     see the counterexample in dwell_kernel_resilience.v).
   - test_lemma1_delta_zero: PROVED (was admitted).
   - test_large_stream: PROVED (was admitted).
   - test_valid_loss_pattern_drop_all: RESTATED with (n <= max_burst_loss)%nat.
     The old claim (drop-all valid for every n when delta = 1) was false: the
     burst constraint caps consecutive drops at max_burst_loss regardless of delta.
   - test_valid_loss_pattern_alternating: RESTATED with (1/2 <= delta).
     The alternating pattern drops 50% of events, so it is only a valid loss
     pattern when the loss budget delta covers that rate. The old
     (max_burst_loss >= 1) premise was irrelevant (every prefix of this pattern
     starts with Keep, so the leading-drop count is always 0).
   - test_complete_resilience_scenario: conclusion restated with INR 10 instead
     of the 10.0 literal (Rplus on literals does not compute, so the old
     `reflexivity` could never close it; INR 10 = 10 in R). *)

Require Import Reals.
From Coq Require Import ZArith.
Require Import Lia.
Require Import Nat.
Require Import Lra.
Require Import List.
Require Import RIneq.
Import ListNotations.

Require Import DwellFiber.dwell_kernel_resilience.

Open Scope R_scope.

(* ========================================================================== *)
(* SECTION 1: Helper Functions for Testing *)
(* ========================================================================== *)

(* Create a simple event for testing *)
Definition make_test_event (dwell_val : R) : event :=
  mk_event (FileRead 1) 0.0 dwell_val 1.

(* Create a stream with n events, each with dwell = 1.0 *)
Fixpoint make_uniform_stream (n : nat) : event_stream :=
  match n with
  | 0 => []
  | S n' => make_test_event 1.0 :: make_uniform_stream n'
  end.

(* Create a loss pattern that keeps all events *)
Definition keep_all_pattern (n : nat) : list loss_pattern :=
  repeat Keep n.

(* Create a loss pattern that drops all events *)
Definition drop_all_pattern (n : nat) : list loss_pattern :=
  repeat Drop n.

(* Create an alternating keep/drop pattern *)
Fixpoint alternating_pattern (n : nat) : list loss_pattern :=
  match n with
  | 0 => []
  | 1 => [Keep]
  | 2 => [Keep; Drop]
  | S n' => Keep :: Drop :: alternating_pattern (n' - 1)
  end.

(* Stream helpers used across tests *)
Lemma uniform_stream_length : forall n : nat,
  length (make_uniform_stream n) = n.
Proof.
  induction n as [|n IH]; simpl.
  - reflexivity.
  - rewrite IH. reflexivity.
Qed.

Lemma uniform_stream_dwell : forall (n : nat) (e : event),
  In e (make_uniform_stream n) -> ev_dwell e = 1.0.
Proof.
  induction n as [|n IH]; intros e Hin.
  - simpl in Hin. contradiction.
  - simpl in Hin. destruct Hin as [Heq | Hin].
    + subst. reflexivity.
    + apply IH. exact Hin.
Qed.

(* Closed form for the uniform test stream's total dwell. *)
Lemma total_dwell_make_uniform : forall n : nat,
  total_dwell (make_uniform_stream n) = INR n.
Proof.
  induction n as [|n IH].
  - simpl. reflexivity.
  - change (1.0 + total_dwell (make_uniform_stream n) = INR (S n)).
    rewrite IH. rewrite S_INR. lra.
Qed.

(* Consecutive-drop counts of patterned prefixes. *)
Lemma ccd_firstn_keep : forall (n' n : nat),
  count_consecutive_drops (firstn n' (repeat Keep n)) = 0%nat.
Proof.
  induction n' as [|n' IH]; intros n.
  - simpl. reflexivity.
  - destruct n as [|n]; simpl; reflexivity.
Qed.

Lemma ccd_firstn_drop : forall (n' n : nat),
  count_consecutive_drops (firstn n' (repeat Drop n)) = Nat.min n' n.
Proof.
  induction n' as [|n' IH]; intros n.
  - simpl. reflexivity.
  - destruct n as [|n].
    + simpl. reflexivity.
    + simpl. rewrite (IH n). lia.
Qed.

(* ========================================================================== *)
(* SECTION 2: Unit Tests for Event Stream Operations *)
(* ========================================================================== *)

(* Test 1: total_dwell of empty stream is 0 *)
Lemma test_total_dwell_empty :
  total_dwell [] = 0.
Proof.
  simpl.
  reflexivity.
Qed.

(* Test 2: total_dwell of single event *)
Lemma test_total_dwell_single :
  total_dwell [make_test_event 5.0] = 5.0.
Proof.
  simpl.
  lra.
Qed.

(* Test 3: total_dwell of multiple events *)
Lemma test_total_dwell_multiple :
  total_dwell [make_test_event 1.0; make_test_event 2.0; make_test_event 3.0] = 6.0.
Proof.
  simpl.
  lra.
Qed.

(* Test 4: apply_loss with keep_all_pattern preserves stream *)
Lemma test_apply_loss_keep_all :
  forall (n : nat),
  apply_loss (make_uniform_stream n) (keep_all_pattern n) = make_uniform_stream n.
Proof.
  intros n.
  unfold keep_all_pattern.
  induction n.
  - simpl. reflexivity.
  - simpl. rewrite IHn. reflexivity.
Qed.

(* Test 5: apply_loss with drop_all_pattern returns empty stream *)
Lemma test_apply_loss_drop_all :
  forall (n : nat),
  apply_loss (make_uniform_stream n) (drop_all_pattern n) = [].
Proof.
  intros n.
  unfold drop_all_pattern.
  induction n.
  - simpl. reflexivity.
  - simpl. rewrite IHn. reflexivity.
Qed.

(* Test 6: apply_loss with alternating pattern *)
Lemma test_apply_loss_alternating :
  apply_loss (make_uniform_stream 4) (alternating_pattern 4) =
  [make_test_event 1.0; make_test_event 1.0].
Proof.
  simpl.
  reflexivity.
Qed.

(* ========================================================================== *)
(* SECTION 3: Unit Tests for Loss Pattern Validation *)
(* ========================================================================== *)

(* Test 7: keep_all_pattern is valid for any stream *)
Lemma test_valid_loss_pattern_keep_all :
  forall (n : nat),
  valid_loss_pattern (make_uniform_stream n) (keep_all_pattern n).
Proof.
  intros n.
  unfold valid_loss_pattern. cbv zeta.
  rewrite (test_apply_loss_keep_all n).
  split.
  - (* Loss rate constraint: no events lost *)
    rewrite Nat.sub_diag.
    assert (H0 : INR 0 = 0) by reflexivity.
    rewrite H0.
    apply Rmult_le_pos.
    + apply delta_pos.
    + apply pos_INR.
  - (* Burst loss constraint: every prefix starts with Keep *)
    intros n' subpattern Hsub.
    subst.
    unfold keep_all_pattern.
    rewrite ccd_firstn_keep.
    pose proof max_burst_positive as Hpos.
    lia.
Qed.

(* Test 8: drop_all_pattern is valid when delta = 1 AND the whole stream fits
   in a single burst window. The burst constraint applies regardless of delta. *)
Lemma test_valid_loss_pattern_drop_all :
  delta = 1 ->  (* Special case: allow 100% loss *)
  forall (n : nat),
  (n <= max_burst_loss)%nat ->
  valid_loss_pattern (make_uniform_stream n) (drop_all_pattern n).
Proof.
  intros Hdelta n Hn.
  unfold valid_loss_pattern. cbv zeta.
  rewrite (test_apply_loss_drop_all n).
  split.
  - (* Loss rate constraint *)
    simpl. rewrite Nat.sub_0_r.
    rewrite Hdelta. rewrite Rmult_1_l.
    apply Rle_refl.
  - (* Burst loss constraint *)
    intros n' subpattern Hsub.
    subst.
    unfold drop_all_pattern.
    rewrite ccd_firstn_drop.
    lia.
Qed.

(* Test 9: Alternating pattern drops half the events, so it needs delta >= 1/2.
   Every prefix starts with Keep, so the burst side needs no extra premise. *)
Lemma test_valid_loss_pattern_alternating :
  (1 / 2 <= delta)%R ->
  valid_loss_pattern (make_uniform_stream 4) (alternating_pattern 4).
Proof.
  intros Hdelta.
  unfold valid_loss_pattern. cbv zeta.
  assert (L4 : length (make_uniform_stream 4) = 4%nat) by (simpl; reflexivity).
  assert (L2 : length (apply_loss (make_uniform_stream 4) (alternating_pattern 4)) = 2%nat).
  { rewrite test_apply_loss_alternating. simpl. reflexivity. }
  split.
  - (* Loss rate constraint: 2 of 4 events dropped, needs delta >= 1/2 *)
    rewrite L4, L2.
    assert (E4 : INR 4 = INR 2 + INR 2).
    { assert (H := plus_INR 2 2). simpl in H. exact H. }
    rewrite E4.
    assert (Hnn : 0 <= INR 2) by apply pos_INR.
    assert (H12 : (1 <= 2 * delta)%R) by lra.
    assert (H := Rmult_le_compat_l (INR 2) 1 (2 * delta) Hnn H12).
    rewrite Rmult_1_r in H.
    replace (delta * (INR 2 + INR 2)) with (INR 2 * (2 * delta)) by ring.
    exact H.
  - (* Burst loss constraint: every prefix starts with Keep *)
    intros n' subpattern Hsub.
    subst.
    assert (Hcc : forall m : nat,
        count_consecutive_drops (firstn m [Keep; Drop; Keep; Drop]) = 0%nat).
    { intros [|m]; simpl; reflexivity. }
    assert (Heq : alternating_pattern 4 = [Keep; Drop; Keep; Drop]).
    { simpl. reflexivity. }
    rewrite Heq. rewrite Hcc.
    pose proof max_burst_positive as Hpos.
    lia.
Qed.

(* ========================================================================== *)
(* SECTION 4: Unit Tests for Lemma 1 - Bounded Loss Preserves Dwell *)
(* ========================================================================== *)

(* Test 10: Lemma 1 holds for keep_all_pattern *)
Lemma test_lemma1_keep_all :
  forall (n : nat),
  let stream := make_uniform_stream n in
  let pattern := keep_all_pattern n in
  valid_loss_pattern stream pattern ->
  (1 - delta) * total_dwell stream <= total_dwell (apply_loss stream pattern).
Proof.
  intros n.
  cbv zeta.
  intros Hvalid.
  apply (bounded_loss_preserves_dwell_bound (make_uniform_stream n) (keep_all_pattern n) _ 1.0).
  - lra.
  - intros e Hin. exact (uniform_stream_dwell _ _ Hin).
  - assumption.
  - reflexivity.
Qed.

(* Test 11: Lemma 1 holds for empty stream *)
Lemma test_lemma1_empty :
  let stream := [] in
  let pattern := [] in
  valid_loss_pattern stream pattern ->
  (1 - delta) * total_dwell stream <= total_dwell (apply_loss stream pattern).
Proof.
  cbv zeta.
  intros Hvalid.
  apply (bounded_loss_preserves_dwell_bound [] [] _ 1.0).
  - lra.
  - intros e Hin. simpl in Hin. contradiction.
  - assumption.
  - reflexivity.
Qed.

(* Test 12: With delta = 0, no loss is allowed.
   PROVED 2026-09-24 (was admitted): the loss-rate constraint forces the
   loss count to 0, hence lengths match, hence apply_loss is the identity. *)
Lemma test_lemma1_delta_zero :
  delta = 0 ->
  forall (stream : event_stream) (pattern : list loss_pattern),
  valid_loss_pattern stream pattern ->
  total_dwell (apply_loss stream pattern) = total_dwell stream.
Proof.
  intros Hdelta stream pattern Hvalid.
  unfold valid_loss_pattern in Hvalid. cbv zeta in Hvalid.
  destruct Hvalid as [Hrate _].
  rewrite Hdelta in Hrate.
  rewrite Rmult_0_l in Hrate.
  assert (Hz : INR (length stream - length (apply_loss stream pattern))%nat = 0).
  { apply Rle_antisym. exact Hrate. apply pos_INR. }
  assert (Hn : (length stream - length (apply_loss stream pattern))%nat = 0%nat).
  { rewrite <- INR_0 in Hz. apply INR_eq in Hz. exact Hz. }
  assert (Heq : apply_loss stream pattern = stream).
  { apply apply_loss_eq_of_length.
    pose proof (length_apply_loss_le stream pattern) as Hle.
    lia. }
  rewrite Heq. reflexivity.
Qed.

(* ========================================================================== *)
(* SECTION 5: Unit Tests for Lemma 2 - Price Monotonicity *)
(* ========================================================================== *)

(* Test 13: update_price_monotonic with equal dwells *)
Lemma test_update_price_monotonic_equal :
  forall (p : price) (d : dwell),
  0 <= p ->
  0 <= d ->
  update_price p d <= update_price p d.
Proof.
  intros p d Hp Hd.
  apply (update_price_monotonic p d d Hp Hd).
  lra.
Qed.

(* Test 14: price_update_monotonic_dwell with identical streams *)
Lemma test_price_update_monotonic_identical :
  forall (p : price) (stream : event_stream),
  0 <= p ->
  update_price_from_stream p stream <= update_price_from_stream p stream.
Proof.
  intros p stream Hp.
  apply price_update_monotonic_dwell.
  - assumption.
  - lra.
Qed.

(* Test 15: Larger dwell leads to larger price (when p + alpha*(d-budget) > 0).
   RESTATED 2026-09-24: premise strengthened to d1 < d2. The old version
   assumed only d1 <= d2 but concluded a strict inequality, which is false
   when d1 = d2. Also fixed the Rmax unfolding (Rmax_left, not Rmax_right). *)
Lemma test_price_increases_with_dwell :
  forall (p : price) (d1 d2 : dwell),
  0 <= p ->
  0 <= d1 ->
  d1 < d2 ->
  p + alpha * (d1 - budget) > 0 ->
  p + alpha * (d2 - budget) > 0 ->
  update_price p d1 < update_price p d2.
Proof.
  intros p d1 d2 Hp Hd1_low Hd12 Hpos1 Hpos2.
  unfold update_price.
  pose proof alpha_pos as Halpha.
  rewrite (Rmax_right _ _ (Rlt_le _ _ Hpos1)).
  rewrite (Rmax_right _ _ (Rlt_le _ _ Hpos2)).
  assert (Hmul : alpha * (d1 - budget) < alpha * (d2 - budget)).
  { apply Rmult_lt_compat_l. exact Halpha. lra. }
  lra.
Qed.

(* ========================================================================== *)
(* SECTION 6: Unit Tests for Lemma 3 - Bounded Price *)
(* ========================================================================== *)

(* Test 16: Price never goes negative *)
Lemma test_price_nonnegative :
  forall (p : price) (stream : event_stream),
  0 <= p ->
  0 <= update_price_from_stream p stream.
Proof.
  intros p stream Hp.
  unfold update_price_from_stream.
  unfold update_price.
  apply Rmax_l.
Qed.

(* Test 17: Price increase is bounded by alpha * total_dwell *)
Lemma test_price_increase_bounded :
  forall (p : price) (stream : event_stream),
  0 <= p ->
  let new_price := update_price_from_stream p stream in
  new_price <= p + alpha * total_dwell stream.
Proof.
  intros p stream Hp.
  cbv zeta.
  unfold update_price_from_stream, update_price.
  apply Rmax_case_strong.
  - (* Rmax returns 0 *)
    intros _.
    pose proof (total_dwell_nonneg stream) as Hnn.
    pose proof alpha_pos as Halpha.
    assert (H : 0 <= alpha * total_dwell stream).
    { apply Rmult_le_pos. lra. exact Hnn. }
    lra.
  - (* Rmax returns the computed value *)
    intros _.
    pose proof alpha_pos as Halpha.
    pose proof budget_is_five as Hbudget.
    assert (H : alpha * (total_dwell stream - budget) <= alpha * total_dwell stream).
    { apply Rmult_le_compat_l. lra. lra. }
    lra.
Qed.

(* Test 18: Lemma 3 holds for keep_all_pattern *)
Lemma test_lemma3_keep_all :
  forall (p : price) (n : nat),
  let stream := make_uniform_stream n in
  let pattern := keep_all_pattern n in
  0 <= p ->
  valid_loss_pattern stream pattern ->
  let final_price := update_price_from_stream p (apply_loss stream pattern) in
  0 <= final_price <= p + alpha * total_dwell stream.
Proof.
  intros p n.
  simpl.
  intros Hp Hvalid.
  apply bounded_price_under_loss.
  - assumption.
  - assumption.
Qed.

(* ========================================================================== *)
(* SECTION 7: Integration Tests *)
(* ========================================================================== *)

(* Test 19: Bridge lemma works correctly *)
Lemma test_bridge_lemma :
  forall (p : price) (stream : event_stream),
  let effective_dwell := total_dwell stream in
  update_price_from_stream p stream = update_price p effective_dwell.
Proof.
  intros p stream.
  unfold update_price_from_stream.
  reflexivity.
Qed.

(* Test 20: Complete resilience scenario.
   RESTATED 2026-09-24: conclusion uses INR 10 (the old 10.0 literal could not
   be reached by reflexivity since Rplus on literals does not compute). *)
Lemma test_complete_resilience_scenario :
  let p := 0.0 in
  let stream := make_uniform_stream 10 in
  let pattern := keep_all_pattern 10 in
  valid_loss_pattern stream pattern ->
  let final_price := update_price_from_stream p (apply_loss stream pattern) in
  final_price = update_price p (INR 10).
Proof.
  intros Hvalid.
  cbv zeta.
  rewrite (test_apply_loss_keep_all 10).
  unfold update_price_from_stream.
  f_equal.
  rewrite total_dwell_make_uniform.
  reflexivity.
Qed.

(* ========================================================================== *)
(* SECTION 8: Performance Tests *)
(* ========================================================================== *)

(* Test 21: Large stream handling. PROVED 2026-09-24 (was admitted). *)
Lemma test_large_stream :
  let n := 1000%nat in
  let stream := make_uniform_stream n in
  let pattern := keep_all_pattern n in
  total_dwell (apply_loss stream pattern) = INR n.
Proof.
  cbv zeta.
  rewrite (test_apply_loss_keep_all 1000).
  rewrite total_dwell_make_uniform.
  reflexivity.
Qed.

(* Test 22: Maximum burst loss constraint *)
Lemma test_max_burst_constraint :
  max_burst_loss = 5%nat ->
  forall (stream : event_stream) (pattern : list loss_pattern),
  valid_loss_pattern stream pattern ->
  (count_consecutive_drops pattern <= 5)%nat.
Proof.
  intros Hmax stream pattern Hvalid.
  unfold valid_loss_pattern in Hvalid. cbv zeta in Hvalid.
  destruct Hvalid as [_ Hburst].
  specialize (Hburst (length pattern) pattern (eq_sym (firstn_all pattern))).
  lia.
Qed.

Close Scope R_scope.
