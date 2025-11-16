# Dwell-Fiber V3: Adaptive I/O Pricing Ransomware Defense

Dwell-Fiber is a **formally-verified eBPF-based ransomware defense system** that enforces 
economic costs on suspicious file access patterns using **Weighted I/O Pressure (WIP)** 
pricing and **ADMM optimization** with proven stability guarantees.

## Core Innovation: WIP Metric

Instead of measuring file **dwell time** (which modern ransomware defeats via intermittent 
access), V3 tracks two **I/O rate signals** per process over a 1-second window:

- **TBW** (Total Bytes Written): Volume of data written (MB/s)
- **UFM** (Unique Files Modified): Scattergun indicator (Files/s)

The **Weighted I/O Pressure** metric combines these:
```
WIP(t) = ω₁·TBW(t) + ω₂·UFM(t)
```

Dynamic weight vectors (`ω₁`, `ω₂`) adapt based on **Trust Classification Module (TCM)** 
tier assignment:

| Tier | Profile | ω₁ | ω₂ | Budget | Detection |
|------|---------|----|----|--------|-----------|
| **T1** | Backups/Archives | 0.9 | 0.1 | 12,000 | TBW ≥ 10k MB/s OR UFM ≤ 1k files/s |
| **T1.5** | Dev Builds | 0.55 | 0.45 | 8,000 | UFM ≥ 20k files/s AND TBW ≥ 500 MB/s |
| **T2** | Untrusted (Default) | 0.3 | 0.7 | 4,000 | Catch-all (TBW < 10 MB/s AND UFM ≥ 1k) |

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│ Kernel Layer (eBPF)                                             │
│ • kprobe/vfs_write: Track bytes written per PID                 │
│ • Window aggregation: Per-PID stats over 1.0s intervals         │
│ • Ring buffer: Emit TBW, UFM, timestamp to userspace            │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ Userspace Daemon (Go)                                           │
│ • Event reader: Consume ring buffer io_event structs            │
│ • TCM: Classify PID into tier based on TBW/UFM thresholds       │
│ • ADMM Controller: Update dual price per PID                    │
│   π(t+1) = max(0, π(t) + α·(WIP(t) - budget_tier))             │
│ • Enforcement: Throttle or kill based on price threshold        │
│ • Prometheus metrics: Export WIP, price, tier per PID           │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ Formal Verification (Coq)                                       │
│ • WIP convexity: Prove WIP is convex in (TBW, UFM)             │
│ • Dual price stability: Prove π remains bounded under tier      │
│   weight switches (non-smooth dynamics)                         │
│ • Lyapunov drift: Prove convergence to optimal π* in discrete   │
│   time (1.0s sampling)                                          │
└─────────────────────────────────────────────────────────────────┘
```

## Build & Run

### Prerequisites
```bash
# Ubuntu 25.10 essential symlink
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm

# Install dependencies
sudo apt-get install clang libbpf-dev coq golang-1.24
```

### Build Order
```bash
make bpf      # Compile eBPF → bpf/dwell_monitor.bpf.o
make verify   # Type-check Coq proofs (~180ms)
make daemon   # Build Go binary → bin/dwell-fiber
```

### Run
```bash
sudo ./bin/dwell-fiber
# Monitor metrics at http://localhost:9090
```

## Documentation

- **[ARCHITECTURE.md](ARCHITECTURE.md)** – Detailed design of three-layer system
- **[V3_MIGRATION.md](V3_MIGRATION.md)** – V0→V3 pivot: from dwell to WIP
- **[FORMAL_VERIFICATION.md](FORMAL_VERIFICATION.md)** – Coq proofs and lemma structure
- **[CONFIGURATION.md](CONFIGURATION.md)** – Tuning alpha, budgets, thresholds
- **[TESTING.md](TESTING.md)** – Unit, formal, integration test suites

## Key Parameters

| Parameter | Default | Range | Role |
|-----------|---------|-------|------|
| `alpha` | 0.6 | (0, 2) | ADMM step size (proven stable) |
| `Δt` | 1.0s | — | I/O measurement window |
| `throttle_price` | 500 | — | Price threshold for process throttle |
| `kill_price` | 1000 | — | Price threshold for process kill |

## Testing

```bash
# Unit tests
go test ./...

# Formal verification
coqchk coq/dwell_stable.vo

# Integration (requires VM with sudo)
sudo test/run_e2e.sh
```

## License

[License text here]

## Contributing

See CONTRIBUTING.md for development guidelines.
