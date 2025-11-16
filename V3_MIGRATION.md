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

## References

- [ARCHITECTURE.md](ARCHITECTURE.md) – System design details
- [FORMAL_VERIFICATION.md](FORMAL_VERIFICATION.md) – Coq proof structure
- `coq/dwell_stable.v` – Formal lemmas (WIP convexity, Lyapunov drift)
- [V3_PIVOT_RESEARCH_DOSSIER.md](V3_PIVOT_RESEARCH_DOSSIER.md) – Detailed research analysis for the pivot
