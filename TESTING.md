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

---

## Enforcement Testing

### Overview

The enforcement system has three components:

1. **Throttling**: Limits process CPU to percentage (e.g., 20%) when dwell > 5s
2. **Killing**: Terminates process when dwell > 15s (ransomware defense)
3. **Safety Checks**: Protects critical system processes from being throttled/killed

### Test Modes

**Mode 1: Dry-Run Testing (Default)**
- No actual enforcement (throttling/killing)
- Logs what enforcement actions *would* happen
- Safe to run on production systems
- Best for initial validation

**Mode 2: Simulation Testing**
- Generates synthetic file access patterns
- Tests enforcement decision-making
- No real processes affected
- Good for algorithm validation

**Mode 3: Real Mode Testing**
- Actually throttles processes
- Actually kills processes
- Requires careful setup and testing
- For production validation

### Running Enforcement Tests

#### Test 1: Dry-Run Enforcement Logic (Safest)

```bash
# Build with enforcement support
make daemon

# Run with dry-run (default, no actual enforcement)
./bin/dwell-fiber-daemon --test-enforcement
```

Expected output:
```
============================== ENFORCEMENT TEST ==============================
ENFORCEMENT TEST: Idle Operations (No Enforcement)
Description: Processes with <1s dwell should not trigger any enforcement
[1/3] PID=3000, Dwell=0.50s -> Expected: none
      Current stats: Throttled=0, Killed=0
      ✅ PASS: No enforcement (as expected)
```

#### Test 2: Simulation Mode with Enforcement Logging

```bash
# Run simulation with enforcement enabled (dry-run)
./bin/dwell-fiber-daemon --simulate

# In another terminal, watch the logs
tail -f /var/log/syslog | grep -i "throttle\|kill\|enforcement"

# Or monitor metrics
watch -n 1 'curl -s http://localhost:9090/metrics | grep dwell_fiber'
```

#### Test 3: Real Enforcement (Carefully!)

⚠️ **Understand the risks**:
- Process throttling can slow down legitimate work
- Process killing terminates programs
- Only run on test systems or with explicit permission

**Create a test process:**
```bash
# Terminal 1: Create a long-running process
cd /tmp
yes "test data" > testfile.txt &
TEST_PID=$!

# Keep the file open for testing
while true; do
    cat testfile.txt > /dev/null
    sleep 0.1
done &
```

**Start daemon with enforcement (DRY-RUN):**
```bash
# This will log enforcement decisions WITHOUT actually enforcing
./bin/dwell-fiber-daemon --simulate --enable-enforcement
```

**Enable real enforcement (DANGEROUS!):**
```bash
# This ACTUALLY throttles/kills processes
./bin/dwell-fiber-daemon --simulate --enable-enforcement --enable-killing

# Watch for enforcement in logs
tail -f /var/log/syslog | grep -i "throttle\|kill"
```

### Enforcement Test Scenarios

| Scenario | Dwell Time | Expected Enforcement |
|----------|-----------|---------------------|
| Idle Operations | < 1s | None |
| Normal Operations | ~5s | None |
| Throttle Threshold | 6-8s | Throttling triggered |
| Kill Threshold | 15+s | Process killed |
| Ransomware Pattern | 2s → 5s → 10s → 15s+ | Throttle → Kill progression |

### Safety Checks

Protected processes that **cannot be throttled or killed**:
```
init, systemd, sshd, dbus-daemon, NetworkManager, gdm, Xorg, wayland
```

To test safety:
```bash
# Try to enforce on init (PID=1)
# Should see: "Safety check: Cannot enforce on protected process init"
```

### Enforcement Metrics

Monitor enforcement activity:
```bash
curl -s http://localhost:9090/metrics | grep enforcement

# Expected output:
dwell_fiber_throttled_processes 5
dwell_fiber_killed_processes 2
dwell_fiber_enforcement_mode 1  # 1=enabled, 0=disabled
```

### Troubleshooting Enforcement

**"Throttle failed: cannot throttle: process no longer exists"**
- **Cause**: Simulation using fake PIDs (3000+)
- **Solution**: This is expected in simulation mode
- **Fix**: Use real BPF mode with actual processes

**"Safety check: Cannot enforce on protected process"**
- **Cause**: Trying to enforce on a critical system process
- **Solution**: Use test processes, not system processes

**No enforcement events logged**
- **Cause**: Enforcement not enabled
- **Solution**: Add `--enable-enforcement` flag
- **Verify**: Check with `./bin/dwell-fiber-daemon --test-enforcement`

### Validation Checklist

- [ ] Dry-run mode shows enforcement decisions without enforcing
- [ ] Simulation loop generates all 4 scenario types
- [ ] Throttle threshold (5s) triggers throttling
- [ ] Kill threshold (15s) triggers killing
- [ ] Protected processes are never enforced
- [ ] Price oscillates correctly
- [ ] Metrics update in real-time
- [ ] Dashboard shows enforcement stats

---

## References

- `daemon/test_suite.go` - Scenario simulation
- `daemon/bpf_monitor.go` - BPF event processing and filtering
- `daemon/enforcement_test.go` - Enforcement test scenarios
- `pkg/enforcement/enforcer.go` - Enforcement implementation
- `pkg/enforcement/throttler.go` - CPU throttling
- `pkg/enforcement/killer.go` - Process killing
- `pkg/enforcement/safety.go` - Safety checks
- `test/test-suite.sh` - Integration orchestration
- `test/workload_generator.go` - Workload creation
- `.github/copilot-instructions.md` - Development guide
