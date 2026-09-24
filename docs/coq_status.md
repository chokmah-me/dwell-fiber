# Coq Formal Verification Status

**Last Updated**: 2026-09-24
**Coq Version**: 8.18.0 (verified; earlier docs claimed "9.1+", which is untested)
**Compilation Status**: ✅ All 4 files compile via `make verify` (exit 0)
**Proof Completion**: 100% — 76 declarations, 0 admitted (was 19 admitted)

---

## Proof Status Summary

| File | Declarations | Admitted | Status |
|------|--------------|----------|--------|
| **dwell_stable.v** | 19 (7 lemmas, 12 theorems) | 0 | ✅ complete |
| **dwell_kernel_resilience.v** | 14 (13 lemmas, 1 theorem) | 0 | ✅ complete |
| **dwell_extended.v** | 16 (8 lemmas, 8 theorems) | 0 | ✅ complete |
| **test_resilience.v** | 27 (helpers + tests) | 0 | ✅ complete |
| **TOTAL** | **76** | **0** | ✅ |

All files also pass `coqchk` independently. `Print Assumptions` on every main
theorem shows only the declared project parameters/axioms plus the Coq 8.18
Reals baseline (`ClassicalDedekindReals.sig_forall_dec`,
`FunctionalExtensionality.functional_extensionality_dep`). No hidden admits.

---

## Corrected statements (2026-09-24)

Several admitted theorems were **false as stated**, not just unproven. They were
corrected rather than admitted. Counterexamples are recorded in the source files.

**dwell_stable.v**
- `convergence_to_budget`, `liveness_normal_mode`: premise `d <= budget`
  weakened to the false claim; corrected to `d < budget`. Counterexample:
  `alpha=1, budget=5, d=5, p=10, epsilon=1` — the price stays 10 forever,
  never entering any epsilon-ball of the budget.

**dwell_kernel_resilience.v**
- `bounded_loss_preserves_dwell_bound`: the original claim (event-count loss
  implies proportional dwell preservation) is false without uniform per-event
  dwell. Counterexample: one event with dwell 100 plus 99 events with dwell
  0.01; dropping 50% including the large event leaves dwell 0.50, not the
  claimed ~50.49. The theorem now takes an explicit uniform-dwell premise;
  the pure event-count bound lives separately in `kept_count_bound`.
- `admm_resilience_to_event_loss`: the original claim (lossy price within
  arbitrary epsilon of budget for any valid pattern) is false.
  Counterexample: `alpha=1, budget=5`, initial price 5, empty stream — the
  update drives the price to 0, deviation 5 regardless of epsilon. Restated
  as a Lipschitz bound: loss perturbs the price by at most `alpha × (dwell
  lost)` relative to the lossless trajectory.

**dwell_extended.v**
- `liveness_normal_operation`: now concludes arrival at a
  policy-generated terminal clean state (the original made unsupported
  claims about record flags); requires `d < budget`.
- `no_livelock`: requires `current_dwell <> budget` (at `d = budget` the
  price is a fixed point and a process can sit between the thresholds forever).
- `fair_pricing_theorem`: requires every process's flags to match the
  deterministic enforcement policy (fairness is a property of the policy,
  not of arbitrary flag assignments).
- `process_safety_nonempty`: the original (`pid > 0 -> pid < 65536`) is
  unprovable — no PID upper bound exists in the model. The honest version
  takes the OS-enforced bound as an explicit premise and concludes
  `(0 < pid < 65536)%nat`.

**test_resilience.v** (was not in the Makefile; now wired in)
- `test_valid_loss_pattern_drop_all`: restated with `(n <= max_burst_loss)%nat`.
  Dropping everything was claimed valid for all `n` when `delta = 1`, but the
  burst constraint caps consecutive drops regardless of `delta`.
- `test_valid_loss_pattern_alternating`: restated with `(1/2 <= delta)`. The
  alternating pattern drops 50% of events, so it needs a matching loss budget;
  the old `max_burst_loss >= 1` premise was irrelevant.
- `test_price_increases_with_dwell`: premise strengthened to `d1 < d2` (the
  strict conclusion was false for `d1 = d2`); fixed `Rmax` unfolding.
- `test_complete_resilience_scenario`: conclusion uses `INR 10` (the `10.0`
  literal is unreachable by `reflexivity` since `Rplus` on literals does not
  compute).

---

## Vacuous placeholders

Three closed declarations are true but substantively empty; they are kept
for API compatibility, not as evidence of verification depth:
- `dwell_stable.v`: `fairness_enforcement_symmetric` (`P <-> P`, reflexive).
- `dwell_stable.v`: `fairness_identical_processes` (bare congruence).
- `dwell_kernel_resilience.v`: `lossy_stream_stability_bridge` (true by
  definitional unfolding).

---

## How to Verify

```bash
cd coq
make clean && make verify   # exit 0; all 4 files
coqchk -R . DwellFiber DwellFiber.dwell_stable
coqchk -R . DwellFiber DwellFiber.dwell_kernel_resilience
coqchk -R . DwellFiber DwellFiber.dwell_extended
coqchk -R . DwellFiber DwellFiber.test_resilience
grep -rn "Admitted\|\badmit\b" --include="*.v" .   # no matches
```

---

**Status**: All proofs complete ✅ (76/76, 0 admitted) under Coq 8.18.0.

For more details, see:
- `COQ_INSTALLATION.md` - Setup guide
- `docs/coq_integration_guide.md` - Proof strategies
- `docs/coq-ebpf-proof-failures.md` - Advanced techniques
- `docs/archived/DEV-NOTES.md` - Historical debugging notes
