# V0 → V3 Migration: From Dwell Time to Weighted I/O Pressure

## The V0 Flaw

Dwell-Fiber V0 measured **file dwell time** ($\text{dwell}$) – the duration a process 
kept a file open. Modern ransomware (LockBit, BlackCat) defeats this via **intermittent 
encryption**: rapidly opening/closing files in sub-second bursts.

**Security bypass**: A ransomware process encrypting 10,000 files per second, each held 
open for 50ms, reports zero dwell time to V0, bypassing the defense entirely.

## The V3 Solution: WIP Metric

Replace the flawed dwell signal with **rate-based metrics** that capture the signature 
of malicious I/O:

```
WIP(t) = ω₁·TBW(t) + ω₂·UFM(t)
```

### Why This Works

1. **TBW (Total Bytes Written)** is **volume-based**, not latency-based
   - High in legitimate backups (10k+ MB/s)
   - Low in ransomware (typically <1000 MB/s on commodity hardware)
   - Cannot be defeated by intermittent access

2. **UFM (Unique Files Modified)** is **scattergun indicator**
   - High in ransomware: touches 1000s of distinct files/sec
   - High in dev builds: compilation can touch 20k+ files/sec
   - Distinguishable via TCM tiering

## Architecture Changes

### V0 Design
```
sys_openat → {pid, inode} pair
sys_close  → calculate dwell = close_ts - open_ts
             emit dwell value to ringbuf
             compute price π(t+1) = max(0, π(t) + α·(dwell - 5.0s))
```

### V3 Design
```
vfs_write  → accumulate bytes_written, track file inodes per 1.0s window
             at window boundary: emit {TBW, UFM}
TCM        → classify tier based on thresholds
ADMM       → compute WIP = ω₁·TBW + ω₂·UFM
             compute price π(t+1) = max(0, π(t) + α·(WIP - B_tier))
Tiers      → dynamic ω and budget allocation per trust level
```

## Parameter Mapping

| Aspect | V0 | V3 |
|--------|----|----|
| **Metric** | File dwell time | TBW + UFM (weighted sum) |
| **Sampling** | Event-driven (open/close) | Periodic (1.0s window) |
| **Budget** | Fixed 5.0s | Dynamic per tier (4k–12k) |
| **Weights** | N/A | Adaptive (ω₁, ω₂) per tier |
| **Update frequency** | Per file close | Per 1.0s window per PID |

## Mathematical Equivalence

Both V0 and V3 solve the same **Lagrangian dual problem** from NUM theory:

```
max_π  g(π) = Σᵢ -πᵢ·(Bᵢ - dᵢ(t))
s.t.   πᵢ ≥ 0
```

where:
- **V0**: dᵢ(t) = dwell time (continuous, per file)
- **V3**: dᵢ(t) = WIP (rate-based, per PID per window)

Both use ADMM dual ascent: `π(t+1) = max(0, π(t) + α·(d(t) - B))`.

The **proof technique** changes (discrete vs. continuous Lyapunov), but 
the **convergence guarantee** is preserved.

## Migration Checklist

- [x] Replace `sys_openat`/`sys_close` hooks with `kprobe/vfs_write`
- [x] Implement per-PID windowed aggregation (TBW, UFM)
- [x] Implement TCM tier classification logic
- [x] Update ADMM update to use WIP metric and dynamic budgets
- [x] Implement new Coq lemmas for discrete-time WIP dynamics
- [x] Update daemon event reader for new ringbuf payload
- [x] Update Prometheus metrics for WIP, tier, price
- [x] Update documentation (README, ARCHITECTURE, etc.)

## Testing Strategy

### Unit Tests (Go)
- `TestClassifyTier`: Verify tier assignment thresholds
- `TestWIPCalculation`: Verify ω₁·TBW + ω₂·UFM arithmetic
- `TestADMMUpdate`: Verify π update formula

### Formal Tests (Coq)
- `test_wip_convexity`: Verify WIP convexity lemma
- `test_dual_bounded`: Verify price boundedness under switches
- `test_lyapunov_drift`: Verify discrete-time drift bound

### Integration Tests (E2E)
- Simulate Normal, Attack, Recovery, Idle workload scenarios
- Verify that Attack scenario (high UFM) is correctly classified and priced
- Verify that Backup scenario (high TBW) remains in T1

## Rollback Plan

If V3 proves unstable:
1. Keep V0 code branch (`git checkout v0/main`)
2. Revert daemon to old event handler (look for `handleDwellEvent` in git log)
3. Restore old BPF hooks and ringbuf payload structure
4. Redeploy: `make clean && make bpf && make daemon && sudo make reload`

Expected rollback time: < 5 minutes with pre-compiled V0 artifacts.

## References

- [ARCHITECTURE.md](ARCHITECTURE.md) – System design details
- [FORMAL_VERIFICATION.md](FORMAL_VERIFICATION.md) – Coq proof structure
- `coq/dwell_stable.v` – Formal lemmas (WIP convexity, Lyapunov drift)
