# Dwell-Fiber Test Suite Architecture

## Overview

The test suite has been architected to provide multi-level testing and validation of the ADMM-based ransomware defense system:

1. **Unit Testing** - In-process scenario simulation
2. **Integration Testing** - BPF + daemon + workload testing
3. **Workload Generation** - Synthetic high-dwell file operations
4. **Metrics Validation** - Real-time dashboard and Prometheus metrics

## Components

### 1. Test Suite in Daemon (`daemon/test_suite.go`)

Provides programmatic test scenario generation and simulation:

```go
// Generate test scenarios
scenarios := GenerateTestScenarios()

// Run each scenario
for _, scenario := range scenarios {
    SimulateScenario(controller, scenario)
}
```

**Four Test Scenarios:**

- **🟢 Normal**: Dwell oscillates 3-7 seconds around budget (5s)
  - Price increases when dwell > 5s
  - Price decreases when dwell < 5s
  - Expected: Stable oscillation pattern

- **🔴 Attack**: Sustained high dwell 7-9 seconds
  - Simulates ransomware behavior
  - Price rises continuously
  - Expected: Price reaches critical threshold

- **🟡 Recovery**: Gradually decreasing dwell 9s → 3s
  - System recovers from attack
  - Price decays exponentially
  - Expected: Smooth convergence to normal

- **⚪ Idle**: Low activity 1-2 seconds
  - Price drops to zero
  - No enforcement needed
  - Expected: Minimal system overhead

### 2. BPF Monitor Wrapper (`daemon/bpf_monitor.go`)

Connects real BPF events to the controller:

```
Kernel (eBPF)
    ↓ (ring buffer events)
BPFMonitor.processEvents()
    ↓ (filter noise < 0.1s)
Controller.HandleCloseEvent()
    ↓ (filter < 2s)
Enforcer.Enforce()
    ↓ (dry-run logs)
Metrics/Dashboard
```

**Key Features:**
- Noise filtering: Events < 0.1s automatically dropped
- Second-level filtering: Events < 2s skipped in controller
- Event logging with clear enforcement decisions
- Automatic metrics updates

### 3. Integration Test Script (`test/test-suite.sh`)

Orchestrates full end-to-end testing:

```bash
./test/test-suite.sh
```

**Steps:**
1. Build daemon if needed
2. Start daemon in background
3. Generate high-dwell workload
4. Collect metrics from /metrics endpoint
5. Check for enforcement events in logs
6. Display Firefox dashboard URL

**Expected Output:**
```
✓ Build: Success
✓ Daemon: Running
✓ Workload: Generated
✓ Metrics: Available
```

### 4. Workload Generator (`test/workload_generator.go`)

Creates synthetic file operations with controlled dwell times:

```go
wg := NewWorkloadGenerator("/tmp/dwell-fiber-workload")
wg.GenerateHighWorkload(3)  // 7-second operations
wg.GenerateCriticalWorkload(2)  // 9-second operations
```

**Run as standalone:**
```bash
cd test && go run workload_generator.go
```

## Testing Workflows

### Quick Start (Simulation Mode - No root required)

```bash
# Build
make daemon

# Terminal 1: Start daemon in simulation
./bin/dwell-fiber-daemon --simulate

# Terminal 2: Monitor
firefox http://localhost:9090

# Terminal 3: Generate workload
cd test && go run workload_generator.go
```

### Real BPF Mode (Requires root + CAP_BPF)

```bash
# Setup (Ubuntu 25.10 - CRITICAL)
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm

# Build all
make all

# Terminal 1: Start daemon with real BPF
sudo ./bin/dwell-fiber-daemon --alpha=0.5 --budget=5.0

# Terminal 2: Monitor
firefox http://localhost:9090

# Terminal 3: Generate workload
cd test && go run workload_generator.go

# Terminal 4: Watch for enforcement
grep "High dwell\|Throttle\|Kill" /var/log/syslog
```

### Full Integration Test

```bash
./test/test-suite.sh
```

**Validates:**
- ✓ Build succeeds
- ✓ Daemon starts
- ✓ BPF loads and captures events
- ✓ Workload generates high dwell
- ✓ Metrics endpoint responds
- ✓ Enforcement decisions logged
- ✓ Firefox dashboard accessible

## Event Filtering

Events are filtered at multiple levels:

1. **BPFMonitor (< 0.1s)**: Automatically drop noise
   ```
   2025/11/02 19:32:30 📊 Event details: Dwell=0.000s
   2025/11/02 19:32:31 📊 Event details: Dwell=0.001s
   [These are silently dropped]
   ```

2. **Controller (< 2s)**: Skip low-impact events
   ```go
   if dwell < 2*time.Second {
       return // Silently skip
   }
   ```

3. **Enforcement (> thresholds)**: Trigger actions
   - Throttle: dwell > 5.0s
   - Kill: dwell > 15.0s

**Expected Log Output:**

```
[Events < 0.1s] [Not logged]
[Events 0.1s - 2s] [Not logged]
[Events > 2s] ⏱️ High dwell: PID=12345 (bash) dwell=7.00s
              🐌 [DRY-RUN] Would throttle PID=12345 (bash) -> 20% CPU
[Events > 15s] 💀 [DRY-RUN] Would kill PID=12346 (bash)
```

## Debugging

### No events appearing?

```bash
# 1. Check BPF loading
dmesg | grep -i "dwell\|ebpf"

# 2. Verify symlink (Ubuntu 25.10 only)
ls -la /usr/include/asm

# 3. Check capabilities
getcap bin/dwell-fiber-daemon

# 4. Test simulation first
./bin/dwell-fiber-daemon --simulate
```

### Enforcement not triggering?

```bash
# Check thresholds
grep -n "ThrottleThreshold\|KillThreshold" daemon/controller.go

# Verify enforcement is enabled
grep -n "enfConfig.Enabled" daemon/controller.go

# Check whitelist (protected processes)
grep -n "Protected:" daemon/main.go
```

### Metrics not updating?

```bash
# Check endpoint
curl http://localhost:9090/metrics

# Verify registration
grep -n "MustRegister" daemon/controller.go

# Check HTTP server
netstat -tuln | grep 9090
```

## Release Criteria for Beta

- [x] Build succeeds on Ubuntu 25.10+
- [x] Coq proofs verify
- [x] BPF program compiles
- [x] Daemon runs in both simulation and real BPF modes
- [x] Enforcement decisions logged clearly
- [x] Metrics exposed via Prometheus + web UI
- [x] Test scenarios demonstrate ADMM convergence
- [x] Workload generator creates measurable dwell times

## Performance Benchmarks

| Metric | Expected | Status |
|--------|----------|--------|
| BPF overhead | <100μs | ✓ |
| Event filtering | <1ms | ✓ |
| Enforcement latency | <50ms | ✓ |
| Metrics export | <10ms | ✓ |

## CI/CD Integration

```yaml
# GitHub Actions example
- name: Build
  run: make all
  
- name: Verify
  run: make verify
  
- name: Test
  run: make test
```

## References

- `daemon/test_suite.go` - Scenario simulation  
- `daemon/bpf_monitor.go` - BPF event processing and filtering
- `test/test-suite.sh` - Integration orchestration
- `test/workload_generator.go` - Workload creation
- `.github/copilot-instructions.md` - Development guide
