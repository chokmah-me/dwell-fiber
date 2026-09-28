# WSL Throttle Test: Debugging Notes (2026-09-28)

## What works
- **`/proc/<pid>/io` fallback monitor** (`daemon/proc_io_monitor.go`): Polls per-process
  `write_bytes` every second, computes MB/s deltas, feeds `ControllerV3.HandleWIPSample`.
  Verified: Python writing 2MB x30 showed `dwell_fiber_v3_tbw 1.9`.
- **Go controller logic** (commits `5d4ab07`, `26896d6`):
  - `HandleWIPSample` tracks price for ALL PIDs, including dead ones (I/O happened).
  - Throttler gracefully skips dead PIDs (returns nil, not error).
  - `SafetyChecker` is mockable via `SafetyCheckerInterface`.
- **BPF write-handler fallback** (`26896d6`): `handle_write_enter` now uses `wip_get`
  (lookup-or-create) instead of lookup-only. Harmless, but not the fix.

## What does NOT work
- **BPF `sys_enter_openat` / `sys_enter_write` tracepoints on WSL**: They attach
  successfully (`✓ Attached to sys_enter_openat`) but never fire for user processes.
  Only system daemons (low PIDs) appear. This is a WSL kernel (6.18.33.2-microsoft-standard)
  limitation, not our code. A full `wsl --shutdown` restart did not fix it.
- **The 7-gate throttle test** cannot pass on BPF alone on this WSL kernel.

## Why we wasted 2 hours
1. **Wrong theory: PID reuse / ghost PIDs** (`986637c`). The daemon tried to throttle
   dead PIDs at 14:24. I assumed BPF was reporting stale data. Actually, the PIDs
   were dead because Python is short-lived; the throttle *should* have been skipped
   gracefully, not treated as an error. The fix was in the enforcer, not the sampler.
2. **Wrong theory: `openat2` bypass** (`06e69c3`). I assumed Python used `openat2`.
   Strace proved Python uses `openat`. The hook was harmless but irrelevant.
3. **Wrong theory: BPF map full / stale entries**. BPF maps are fresh on daemon start.
   The issue was tracepoints not firing, not map state.
4. **Missed the obvious**: The `dwell_fiber_v3_tbw 0` after a 100KB Python write
   proved the BPF path was blind. I should have pivoted to `/proc` immediately
   instead of debugging BPF internals for an hour.
5. **Test used dead PIDs**: The enforcer tests used `NoSuchPID` (1<<30). My graceful-skip
   fix broke them. Made `SafetyChecker` mockable instead.

## Key lesson
When the sensor (BPF) is blind, fix the sensor path first. Don't debug the
controller logic when the input is zero. A 30-second `strace` + metric check
(`v3_tbw 0` after write) proves sensor blindness; pivot to a working sensor
(`/proc`) immediately.

## Commits
- `5d4ab07`: Fix V3 Go logic (track all PIDs, skip dead enforcement gracefully)
- `26896d6`: BPF write handler lookup-or-create (harmless, not the fix)
- `a599ff1` + `323bdf9`: `/proc` I/O fallback monitor (THE FIX)
