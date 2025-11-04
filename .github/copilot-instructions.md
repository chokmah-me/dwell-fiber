# Dwell-Fiber: AI Coding Agent Instructions

## Project Overview

Dwell-Fiber is a formally-verified eBPF-based ransomware defense system that prevents attacks by enforcing economic costs on file access patterns. The system monitors file "dwell time" (how long processes keep files open) and uses ADMM optimization with mathematical stability guarantees.

## Core Architecture

### Three-Layer Design
1. **Kernel (eBPF)**: `bpf/dwell_monitor.bpf.c` tracks sys_openat/sys_close, measures dwell times
2. **Userspace (Go)**: `daemon/` implements ADMM controller with proven-stable pricing algorithm  
3. **Formal Verification (Coq)**: `coq/dwell_stable.v` provides mathematical stability proofs

### Key Data Flow
- eBPF → Ring Buffer → Go Controller → ADMM Price Updates → Process Enforcement
- Price formula: `price(t+1) = max(0, price(t) + α×(dwell(t) - budget))`
- Enforcement: throttling at medium prices, killing at high prices via `pkg/enforcement/`

## Critical Build Requirements

### Ubuntu 25.10 Symlink Fix (ESSENTIAL)
```bash
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm
```
**Without this symlink, eBPF compilation fails.** Always check this first when encountering build issues.

### Build Order Dependencies
1. `make bpf` - Compile eBPF program (requires clang, libbpf-dev)
2. `make coq` - Verify mathematical proofs (~180ms, requires coq 8.18+)
3. `make daemon` - Build Go binary (depends on bpf step)

## Development Patterns

### ADMM Controller (`daemon/controller.go`)
- Central state management with `sync.RWMutex` for thread safety
- Simulation mode when BPF loading fails (graceful degradation)
- Four scenario patterns: Normal, Attack, Recovery, Idle - each with specific dwell time ranges
- Metrics exported to Prometheus on port 9090 with built-in web UI

### Enforcement Pipeline (`pkg/enforcement/`)
- Safety checks before any process actions
- Configurable thresholds: throttle vs kill decisions  
- PID tracking with cleanup routines for stale processes
- Whitelist patterns for system processes (avoid killing init, kernel threads)

### BPF Event Processing
- Maps: `dwell_tracker` (process state), `events` (ring buffer)
- Key structure: `{pid, inode}` pairs for file tracking
- Event emission on file close with calculated dwell duration

## Testing Approach

### Multi-Level Verification
- **Unit**: Go package tests with `go test ./...`
- **Formal**: Coq proof verification with `make verify` 
- **Integration**: E2E tests in `test/run_e2e.sh` (requires VM with sudo)
- **Performance**: Syscall benchmarks in `test/syscall_bench.c`

### Simulation vs Real Mode
- Daemon automatically falls back to simulation when BPF loading fails
- Simulation generates synthetic dwell patterns for algorithm testing
- Real mode requires root/CAP_BPF privileges for eBPF operations

## Configuration Parameters

### ADMM Algorithm Tuning
- `alpha`: Step size (0.5 default, proven stable for 0 < α < 2)
- `budget`: Target dwell time (5.0 seconds default)
- Both mathematically verified in Coq proofs

### Enforcement Thresholds
- Configured in `pkg/enforcement/config.go`
- Separate thresholds for throttling vs termination
- Safety checker prevents enforcement on critical system processes

## Debugging Workflows

### BPF Issues
1. Check symlink: `/usr/include/asm` → `/usr/include/x86_64-linux-gnu/asm`
2. Verify kernel version: requires 5.8+ for eBPF support
3. Check capabilities: CAP_BPF required for program loading
4. Use simulation mode to test algorithm without BPF

### Mathematical Verification
- Coq proofs should compile in <1 second
- Use `coqchk` for additional verification checking
- Proof failures indicate parameter violations (check alpha range)

### Metrics & Observability
- Prometheus metrics at `http://localhost:9090/metrics`
- Web UI at `http://localhost:9090` shows real-time price/dwell graphs
- Four scenario buttons for testing algorithm behavior

## Common Gotchas

- **Root privileges**: Required for eBPF loading and process enforcement
- **Go module path**: Use `github.com/dyb5784/dwell-fiber` in imports
- **Concurrent access**: Always use controller mutex when accessing price state
- **Process safety**: Enforcement whitelist prevents killing essential processes
- **Ring buffer**: Events may be dropped under high load - implement backpressure

## File Patterns to Follow

- BPF: SEC() annotations, proper map definitions, ring buffer emission
- Go: Prometheus metrics registration, mutex protection, graceful error handling
- Coq: Parameter axioms, theorem statements with proper scoping
- Tests: Requires sudo for realistic eBPF testing scenarios