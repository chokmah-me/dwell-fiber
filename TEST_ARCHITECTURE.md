# Test Suite Architecture Summary

## Problem Identified

The issue you were experiencing had two root causes:

1. **Missing BPFMonitor Implementation**: The `BPFMonitor` type was referenced in `daemon/main.go` but never implemented, preventing proper connection between BPF events and the controller.

2. **Noise Overload**: All BPF events (including near-zero dwell times) were being logged, drowning out the meaningful high-dwell events. The suggestion to filter events >2 seconds was critical.

## Solutions Implemented

### 1. Created BPFMonitor Wrapper (`daemon/bpf_monitor.go`)

This bridges the gap between the BPF loader and the controller:

```
Kernel eBPF Program
    ↓
BPFManager (pkg/bpf/)
    ↓
BPFMonitor.processEvents() [NEW]
    ↓ (filters < 0.1s noise)
Controller.HandleCloseEvent()
    ↓ (filters < 2s)
Enforcer.Enforce()
    ↓
Metrics & Logs
```

**Key improvements:**
- Automatically filters events < 0.1s (background noise)
- Gracefully skips events < 2s in controller
- Logs high-dwell events with clear formatting
- Connects to enforcement pipeline

### 2. Updated Controller Event Handling (`daemon/controller.go`)

Changed `HandleCloseEvent()` signature to accept dwell time directly:

```go
// Before (incomplete):
func (c *Controller) HandleCloseEvent(pid int) { ... }

// After (complete):
func (c *Controller) HandleCloseEvent(pid int, cmd string, dwell time.Duration) {
    // Filter noise
    if dwell < 2*time.Second {
        return
    }
    
    // Log high-dwell events
    fmt.Printf("⏱️  High dwell: PID=%d (%s) dwell=%.2fs\n", pid, cmd, dwell.Seconds())
    
    // Trigger enforcement
    c.enforcer.Enforce(pid, cmd, dwell)
}
```

### 3. Created Test Suite (`daemon/test_suite.go`)

Programmatic scenario testing with ADMM algorithm validation:

- **🟢 Normal**: 3-7s oscillation (budget = 5s)
- **🔴 Attack**: 7-9s sustained high dwell (ransomware simulation)
- **🟡 Recovery**: 9s → 3s gradual improvement
- **⚪ Idle**: 1-2s minimal activity

Run tests in daemon or integrate into Go test suite.

### 4. Created Integration Test Script (`test/test-suite.sh`)

End-to-end orchestration:

```bash
./test/test-suite.sh
```

Automates:
1. Build daemon
2. Start with BPF
3. Generate workload
4. Collect metrics
5. Validate enforcement

### 5. Created Workload Generator (`test/workload_generator.go`)

Synthetic file operations with controlled dwell times:

```bash
cd test && go run workload_generator.go
```

Generates:
- Idle operations (0.5s)
- Normal operations (5s)
- High dwell (7s) → should trigger throttle
- Critical dwell (9s) → should trigger kill

### 6. Updated TESTING.md

Comprehensive guide including:
- Testing workflows
- Event filtering explanation
- Debugging procedures
- Performance benchmarks
- Release criteria

## Expected Behavior After Changes

### Terminal Output (Real BPF Mode)

```
🛡️  Dwell-Fiber Daemon Starting
   Alpha: 0.50
   Budget: 5.00 seconds
   ✓ BPF program loaded
   ✓ Ring buffer reader started
   ✓ Attached to sys_enter_openat
   ✓ Attached to sys_enter_close

[Noise events are silently filtered]

📥 Received event #1 (size: 304 bytes)
📊 Event details: PID=12345, Cmd=bash, Dwell=7.00s
⏱️  High dwell: PID=12345 (bash) dwell=7.00s
🐌 [DRY-RUN] Would throttle PID=12345 (bash) dwell=7.00s -> 20% CPU
✓ Processed event for PID=12345 (bash)
```

### Firefox Dashboard Updates

- **Dwell Time**: Shows ~6-7 seconds (when high-dwell workload running)
- **Price**: Increases from 0.1 → 0.3 → 0.5+ (ADMM responding)
- **Status**: 🟡 Warning → 🔴 High Dwell (as dwell increases)

### Metrics Endpoint

```bash
$ curl http://localhost:9090/metrics
dwell_fiber_price 0.350000
dwell_fiber_dwell_time 6.500000
dwell_fiber_budget 5.000000
dwell_fiber_violation 1.500000
```

## Testing Quick Reference

### 1. Simulation Mode (No root, no BPF)
```bash
./bin/dwell-fiber-daemon --simulate
# Goes immediately into test scenarios
# Generates synthetic dwell patterns
# No kernel involvement needed
```

### 2. Real BPF Mode (Requires root)
```bash
# Critical Ubuntu 25.10 step
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm

# Build
make all

# Run
sudo ./bin/dwell-fiber-daemon
```

### 3. Generate High-Dwell Workload
```bash
cd test
go run workload_generator.go
# Creates /tmp/dwell-fiber-workload/*
# Keeps files open for 5-9 seconds
```

### 4. Monitor
```bash
# Terminal 1: Daemon logs
tail -f /tmp/dwell-fiber-test/daemon.log

# Terminal 2: Metrics
watch 'curl -s http://localhost:9090/metrics'

# Terminal 3: Dashboard
firefox http://localhost:9090

# Terminal 4: Enforcement logs (real mode)
grep "High dwell\|DRY-RUN" /var/log/syslog
```

## Files Created/Modified

| File | Purpose | Status |
|------|---------|--------|
| `daemon/bpf_monitor.go` | BPF event wrapper | ✓ New |
| `daemon/controller.go` | Event handling + filtering | ✓ Updated |
| `daemon/test_suite.go` | Test scenarios | ✓ New |
| `test/test-suite.sh` | Integration orchestration | ✓ New |
| `test/workload_generator.go` | Synthetic workload | ✓ New |
| `TESTING.md` | Test documentation | ✓ Updated |
| `.github/copilot-instructions.md` | AI coding guide | ✓ Existing |

## Next Steps

1. **Build and test:**
   ```bash
   make clean all
   ./bin/dwell-fiber-daemon --simulate
   ```

2. **Run workload generator:**
   ```bash
   cd test && go run workload_generator.go
   ```

3. **Monitor Firefox dashboard:**
   ```
   http://localhost:9090
   ```

4. **Try real BPF mode** (on Ubuntu 25.10 with root access):
   ```bash
   make clean all
   sudo ./bin/dwell-fiber-daemon
   ```

5. **Run full integration test:**
   ```bash
   ./test/test-suite.sh
   ```

## Known Issues & Workarounds

| Issue | Cause | Solution |
|-------|-------|----------|
| "asm/types.h not found" | Missing symlink (Ubuntu 25.10) | Run: `sudo ./scripts/fix-asm-symlink.sh` |
| BPF program fails to load | Missing capabilities | Run as root: `sudo` |
| No events in logs | Workload not creating files | Run workload generator manually |
| Dashboard shows 0 dwell | Simulation mode active | Disable with `--simulate=false` |
| Events < 2s flooding output | Noise issue | Already fixed - now filtered |

## Architecture Diagrams

### Event Flow
```
Process File Operations
    ↓
Kernel eBPF (tracepoint hooks)
    ↓ Ring Buffer (256KB)
    ↓ (measures dwell time)
BPFMonitor.processEvents() [filters < 0.1s]
    ↓
Controller.HandleCloseEvent() [filters < 2s, logs high dwell]
    ↓
Enforcer.Enforce() [throttle/kill decisions]
    ↓
Prometheus Metrics & Web Dashboard
```

### ADMM Price Update
```
Measured Dwell Time
    ↓
Calculate Violation = dwell - budget
    ↓
ADMM Update: price(t+1) = max(0, price(t) + α × violation)
    ↓
Compare Price vs Thresholds
    ├─ price > 0.5: Critical (kill if enabled)
    ├─ price > 0.2: High (throttle)
    └─ price ≤ 0.2: Normal (no action)
```

This comprehensive architecture ensures:
- **No noise**: Events < 2s are silently dropped
- **Clear visibility**: High-dwell events logged with enforcement decisions
- **Multiple test modes**: Simulation for development, real BPF for validation
- **Proper enforcement**: Dry-run logs show what would happen
- **Metrics visibility**: Real-time dashboard + Prometheus compatibility
