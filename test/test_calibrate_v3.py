#!/usr/bin/env python3
"""
Unit tests for test/calibrate_v3.py (stdlib only).

Run from worktree root:
    python test/test_calibrate_v3.py
"""

from __future__ import annotations

import math
import sys
import unittest
from pathlib import Path

# Allow `python test/test_calibrate_v3.py` without installing a package.
sys.path.insert(0, str(Path(__file__).resolve().parent))

import calibrate_v3 as c  # noqa: E402


class TestFromPeaks(unittest.TestCase):
    def test_pb10_pi100_throttle_between_and_kill_above(self):
        r = c.recommend_from_peaks(10.0, 100.0, margin_throttle=0.15, kill_ratio=2.0)
        thr = r["recommended_V3ThrottlePrice"]
        kill = r["recommended_V3KillPrice"]
        self.assertIsNotNone(thr)
        self.assertIsNotNone(kill)
        assert thr is not None and kill is not None
        self.assertGreater(thr, 10.0)
        self.assertLessEqual(thr, 100.0)
        self.assertGreater(kill, thr)
        self.assertTrue(r["gates"]["benign_below_throttle"])
        self.assertTrue(r["gates"]["intermittent_clears_throttle"])
        self.assertTrue(r["gates"]["kill_above_throttle"])
        self.assertEqual(r["current_defaults"]["V3ThrottlePrice"], 50.0)
        self.assertEqual(r["current_defaults"]["V3KillPrice"], 150.0)
        # Explicit formula check
        expected = 10.0 + 0.15 * (100.0 - 10.0)
        self.assertAlmostEqual(thr, expected)
        self.assertAlmostEqual(kill, expected * 2.0)

    def test_pi_le_pb_infeasible_all_gates_false(self):
        r = c.recommend_from_peaks(50.0, 40.0)
        self.assertFalse(r["gates"]["benign_below_throttle"])
        self.assertFalse(r["gates"]["intermittent_clears_throttle"])
        self.assertFalse(r["gates"]["kill_above_throttle"])
        self.assertIsNone(r["recommended_V3ThrottlePrice"])
        self.assertIsNone(r["recommended_V3KillPrice"])
        joined = " ".join(r["notes"]).lower()
        self.assertIn("infeasible", joined)

        r_eq = c.recommend_from_peaks(10.0, 10.0)
        self.assertFalse(any(r_eq["gates"].values()))


class TestSimulate(unittest.TestCase):
    def test_intermittent_peak_gt_benign_peak(self):
        benign = c.simulate(c.synthetic_profile("benign_tar"), tier="t2")
        attack = c.simulate(c.synthetic_profile("intermittent_attack"), tier="t2")
        self.assertGreater(
            attack["peak_price"],
            benign["peak_price"],
            msg=(
                f"intermittent peak {attack['peak_price']} should exceed "
                f"benign peak {benign['peak_price']}"
            ),
        )
        self.assertGreater(attack["peak_price"], 0.0)

    def test_leak_snap_below_half(self):
        # value that becomes < 0.5 after *0.9 goes to 0
        # 0.5 * 0.9 = 0.45 < 0.5 → snap to 0
        self.assertEqual(c.leak_price(0.5, leak=0.9), 0.0)
        # 0.55 * 0.9 = 0.495 < 0.5 → 0
        self.assertEqual(c.leak_price(0.55, leak=0.9), 0.0)
        # 0.6 * 0.9 = 0.54 >= 0.5 → keep
        self.assertAlmostEqual(c.leak_price(0.6, leak=0.9), 0.54)
        # Exactly at boundary: 0.5 / 0.9 ≈ 0.555... * 0.9 = 0.5 → not < 0.5
        boundary = 0.5 / 0.9
        self.assertAlmostEqual(c.leak_price(boundary, leak=0.9), 0.5)

    def test_update_non_negative(self):
        # large budget deficit should not go negative
        p = c.update_price_v3(0.0, wip=0.0, budget=300.0)
        self.assertEqual(p, 0.0)
        p = c.update_price_v3(10.0, wip=0.0, budget=300.0)
        self.assertGreaterEqual(p, 0.0)

    def test_t2_wip_formula(self):
        # WIP = 0.3*tbw + 0.7*ufm
        self.assertAlmostEqual(c.calculate_wip("t2", 100.0, 200.0), 170.0)


class TestCLISmoke(unittest.TestCase):
    def test_main_from_peaks_exit_0(self):
        rc = c.main(
            [
                "--from-peaks",
                "--benign-peak",
                "10",
                "--intermittent-peak",
                "100",
            ]
        )
        self.assertEqual(rc, 0)

    def test_main_from_peaks_infeasible_exit_0(self):
        rc = c.main(
            [
                "--from-peaks",
                "--benign-peak",
                "100",
                "--intermittent-peak",
                "10",
            ]
        )
        self.assertEqual(rc, 0)

    def test_main_simulate_exit_0(self):
        rc = c.main(["--simulate", "--profile", "intermittent_attack"])
        self.assertEqual(rc, 0)


if __name__ == "__main__":
    # Exit 0 on success (unittest.main uses SystemExit with failure count).
    unittest.main(verbosity=2)
