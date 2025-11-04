# Enforcement System Testing Guide

## Overview

The Dwell-Fiber enforcement system has three components:

1. **Throttling**: Limits process CPU to a percentage (e.g., 20%) when dwell > 5s
2. **Killing**: Terminates process when dwell > 15s (ransomware defense)
3. **Safety Checks**: Protects critical system processes from being throttled/killed

## Test Modes

### Mode 1: Dry-Run Testing (Default)
- No actual enforcement (throttling/killing)
- Logs what enforcement actions *would* happen
- Safe to run on production systems
- Best for initial validation

### Mode 2: Simulation Testing
- Generates synthetic file access patterns
- Tests enforcement decision-making
- No real processes affected
- Good for algorithm validation

### Mode 3: Real Mode Testing
- Actually throttles processes
- Actually kills processes
- Requires careful setup and testing
- For production validation

## Running Tests

### Test 1: Dry-Run Enforcement Logic (Safest)

```bash
cd ~/dwell-fiber

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
...
```

### Test 2: Simulation Mode with Enforcement Logging

```bash
# Run simulation with enforcement enabled (dry-run)
./bin/dwell-fiber-daemon --simulate

# In another terminal, watch the logs
tail -f /var/log/syslog | grep -i "throttle\|kill\|enforcement"

# Or monitor metrics
watch -n 1 'curl -s http://localhost:9090/metrics | grep dwell_fiber'
```

Expected behavior:
- Price oscillates as before
- Logs show enforcement decisions
- No actual processes throttled/killed

### Test 3: Real Enforcement (Carefully!)

**Step 1: Understand the risks**
- Process throttling can slow down legitimate work
- Process killing terminates programs
- Only run on test systems or with explicit permission

**Step 2: Create a test process that keeps files open**

```bash
# Terminal 1: Create a long-running process
cd /tmp
yes "test data" > testfile.txt &
TEST_PID=$!
echo "Test process PID: $TEST_PID"

# Keep the file open for testing
while true; do
    cat testfile.txt > /dev/null
    sleep 0.1
done &

# Get the looping process PID
LOOP_PID=$!
echo "Loop process PID: $LOOP_PID"

# Keep them alive
wait
```

**Step 3: Start daemon with enforcement enabled (DRY-RUN)**

```bash
cd ~/dwell-fiber

# This will log enforcement decisions WITHOUT actually enforcing
./bin/dwell-fiber-daemon --simulate --enable-enforcement
```

You should see logs like:
```
⚠️  Throttle failed: cannot throttle: process no longer exists
[DRY-RUN] Would throttle PID=1234
```

**Step 4: Enable real enforcement (DANGEROUS!)**

```bash
# This ACTUALLY throttles/kills processes
./bin/dwell-fiber-daemon --simulate --enable-enforcement --enable-killing

# Watch for enforcement in logs
tail -f /var/log/syslog | grep -i "throttle\|kill"
```

## Test Scenarios

### Scenario 1: Idle Operations (< 1s dwell)
- Expected: No enforcement
- Test: Quick file opens/closes
- Validation: Stats show 0 throttles, 0 kills

### Scenario 2: Normal Operations (5s dwell)
- Expected: No enforcement
- Test: Files kept open ~5 seconds
- Validation: Price stable, no enforcement

### Scenario 3: Throttle Threshold (6-8s dwell)
- Expected: Throttling triggered
- Test: Files kept open 6-8 seconds
- Validation: Process throttled (if enforcement enabled)

### Scenario 4: Kill Threshold (15+ dwell)
- Expected: Process killed
- Test: Files kept open 15+ seconds
- Validation: Process terminated (if killing enabled)

### Scenario 5: Ransomware Pattern
- Expected: Progressive enforcement
- Test: Dwell time escalates 2s → 5s → 10s → 15s+
- Validation: Throttle → Kill progression

## Safety Checks

Protected processes that **cannot be throttled or killed**:

```
init, systemd, sshd, dbus-daemon, NetworkManager, gdm, Xorg, wayland
```

To test safety:

```bash
# Try to enforce on init (PID=1)
# Should see: "Safety check: Cannot enforce on protected process init"

# Try to enforce on sshd
# Should see: "Safety check: Cannot enforce on protected process sshd"
```

## Validation Checklist

- [ ] Dry-run mode shows enforcement decisions without enforcing
- [ ] Simulation loop generates all 4 scenario types
- [ ] Throttle threshold (5s) triggers throttling
- [ ] Kill threshold (15s) triggers killing
- [ ] Protected processes are never enforced
- [ ] Price oscillates correctly
- [ ] Metrics update in real-time
- [ ] Dashboard shows enforcement stats

## Expected Metrics

When enforcement is active, monitor these:

```bash
curl -s http://localhost:9090/metrics | grep enforcement

# Expected output:
dwell_fiber_throttled_processes 5
dwell_fiber_killed_processes 2
dwell_fiber_enforcement_mode 1  # 1=enabled, 0=disabled
```

## Troubleshooting

### "Throttle failed: cannot throttle: process no longer exists"
- **Cause**: Simulation using fake PIDs (3000+)
- **Solution**: This is expected in simulation mode
- **Fix**: Use real BPF mode with actual processes

### "Safety check: Cannot enforce on protected process"
- **Cause**: Trying to enforce on a critical system process
- **Solution**: Use test processes, not system processes
- **Fix**: Create your own test process

### No enforcement events logged
- **Cause**: Enforcement not enabled
- **Solution**: Add `--enable-enforcement` flag
- **Verify**: Check with `./bin/dwell-fiber-daemon --test-enforcement`

## Next Steps

1. **Run dry-run tests**: `./bin/dwell-fiber-daemon --test-enforcement`
2. **Verify all scenarios pass**
3. **Test with simulation**: `--simulate --enable-enforcement`
4. **Create test harness** for real process enforcement
5. **Document results** in enforcement report

## Files

- `daemon/enforcement_test.go` - Test scenarios and harness
- `pkg/enforcement/enforcer.go` - Enforcement implementation
- `pkg/enforcement/throttler.go` - CPU throttling
- `pkg/enforcement/killer.go` - Process killing
- `pkg/enforcement/safety.go` - Safety checks

---

**Status**: Testing framework ready
**Next**: Run `--test-enforcement` to validate
