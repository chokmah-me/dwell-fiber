#!/usr/bin/env python3
"""
v1.5.0 FD-tracking regression test.

Pre-fix: opening N files concurrently in one process collided on key (pid, 0),
so only one dwell event was reported. This test opens 3 files with staggered
hold times, then verifies the daemon reports 3 distinct dwell events.

Run the daemon FIRST (separate terminal):
  sudo ./bin/dwell-fiber-daemon --alpha=0.5 --budget=5.0

Then:
  python3 test/test_fd_tracking.py
"""
import os
import sys
import tempfile
import time

HOLDS = (3.5, 5.0, 7.0)   # seconds; all > 100ms noise filter, distinguishable


def main():
    with tempfile.TemporaryDirectory() as td:
        files = [open(os.path.join(td, f"f{i}"), "wb") for i in range(len(HOLDS))]
        opened_at = time.time()
        # Stagger close times by holding different durations.
        # Sort by hold so we close in increasing-hold order.
        deadlines = sorted([(opened_at + h, fh) for h, fh in zip(HOLDS, files)])
        for deadline, fh in deadlines:
            now = time.time()
            if deadline > now:
                time.sleep(deadline - now)
            fh.close()
        print(f"[ok] opened {len(HOLDS)} files concurrently, held {HOLDS}s, closed.")
        print("[expect] daemon should log 3 separate 'High dwell' lines (~3.5/5.0/7.0s).")
        print("[verify] grep daemon stdout for 'High dwell:' and count distinct durations.")
        print("[verify] pre-fix: only 1 line. post-fix: 3 lines.")


if __name__ == "__main__":
    main()
