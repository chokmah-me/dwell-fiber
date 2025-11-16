# Dwell-Fiber V3 Architecture

## Layers

1. **Kernel (eBPF)**: kprobe/vfs_write, per-PID TBW/UFM, ringbuf events.
2. **Userspace (Go)**: Event reader, TCM tier classifier, ADMM price update, enforcement.
3. **Formal (Coq)**: Stability proofs for WIP, tier switching, discrete ADMM.

## Data Flow

eBPF → Ring Buffer → Go Controller → ADMM Price Updates → Enforcement

## Kernel Details

- Aggregates TBW/UFM per PID per 1s window.
- Emits io_event struct: {pid, bytes_written, unique_files, timestamp_ns}.

## Userspace Details

- Reads events, classifies tier, computes WIP, updates price:
  ```
  price(t+1) = max(0, price(t) + α·(WIP(t) - budget_tier))
  ```
- Prometheus metrics: WIP, price, tier per PID.

## Formal Layer

- Proves WIP convexity, price boundedness under tier switch, Lyapunov drift.
