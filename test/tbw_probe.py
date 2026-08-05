#!/usr/bin/env python3
"""
TBW gate probe for V3 calibration.

Writes a burst of random-byte files to /tmp/tbwprobe so the write tracepoint's
TBW accumulation can be read from the daemon log, then sleeps to keep the
process alive while the daemon reports its identity.

Usage:
    python3 test/tbw_probe.py           # 1200 x 1MB burst (1.2 GB)
    python3 test/tbw_probe.py --small   # 3000 x 256KB burst (750 MB)
"""
import os
import shutil
import sys
import time

PROBE_DIR = "/tmp/tbwprobe"
DEFAULT_FILES = 1200
DEFAULT_SIZE = 1 << 20
SMALL_FILES = 3000
SMALL_SIZE = 256 << 10
TAIL_SLEEP_S = 5.0


def main() -> int:
    small = "--small" in sys.argv[1:]
    n_files = SMALL_FILES if small else DEFAULT_FILES
    size = SMALL_SIZE if small else DEFAULT_SIZE

    if os.path.isdir(PROBE_DIR):
        shutil.rmtree(PROBE_DIR)
    os.makedirs(PROBE_DIR)

    print(
        f"tbw_probe pid={os.getpid()} files={n_files} size={size}",
        flush=True,
    )
    data = os.urandom(size)
    for i in range(n_files):
        with open(os.path.join(PROBE_DIR, f"p_{i:05d}.dat"), "wb") as f:
            f.write(data)
    print(
        f"tbw_probe wrote {n_files} files ({n_files * size} bytes)",
        flush=True,
    )
    time.sleep(TAIL_SLEEP_S)
    return 0


if __name__ == "__main__":
    sys.exit(main())
