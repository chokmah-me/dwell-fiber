# V0 → V3 Migration: From Dwell Time to Weighted I/O Pressure

## V0 Flaw

Dwell time metric bypassed by ransomware using sub-second file access.

## V3 Solution

Switch to rate-based WIP metric:
```
WIP(t) = ω₁·TBW(t) + ω₂·UFM(t)
```
TCM assigns tier, ADMM updates price per PID.

## Migration Checklist

- [x] Replace open/close hooks with kprobe/vfs_write
- [x] Windowed TBW/UFM aggregation
- [x] TCM tier classifier
- [x] ADMM update for WIP/budget
- [x] Coq lemmas for discrete-time WIP
- [x] Update docs and metrics

## Rollback

Restore V2 via backup branch if needed.
