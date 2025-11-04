# Common Issues & Solutions

## Build Issues

### Error: "asm/types.h: No such file or directory"

**Cause**: Ubuntu 25.10 requires symlink fix for eBPF compilation

**Solution**:
```bash
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm
```

**Verify**:
```bash
ls -la /usr/include/asm
# Should show: asm -> /usr/include/x86_64-linux-gnu/asm
```

---

### Error: "package github.com/cilium/ebpf: no Go files in..."

**Cause**: Missing Go dependencies

**Solution**:
```bash
go mod download
go mod tidy
make daemon
```

---

### Error: "coqc: not found"

**Cause**: Coq not installed

**Solution**:
```bash
sudo apt-get install coq
# Verify
coq -v
```

---

## Runtime Issues

### Problem: "Permission denied" when starting daemon

**Cause**: BPF loading requires root or CAP_BPF capability

**Solution**:
```bash
# Option 1: Use sudo
sudo ./bin/dwell-fiber-daemon

# Option 2: Grant CAP_BPF capability
sudo setcap cap_bpf=ep ./bin/dwell-fiber-daemon
./bin/dwell-fiber-daemon
```

---

### Problem: "Daemon starts but no BPF events appear"

**Cause**: BPF program may not be loading, or events are filtered out

**Solution** (in order):
```bash
# 1. Check if using simulation mode (no BPF)
./bin/dwell-fiber-daemon --simulate
# If this works, BPF is the issue

# 2. Check BPF loading in logs
dmesg | grep -i "dwell\|ebpf" | tail -10

# 3. Verify symlink again
ls -la /usr/include/asm

# 4. Check kernel version (needs 5.8+)
uname -r

# 5. Try explicit BPF object path
sudo ./bin/dwell-fiber-daemon --bpf-obj=bpf/dwell_monitor.bpf.o

# 6. Rebuild eBPF
make clean all
```

---

### Problem: "Daemon runs but no high-dwell events logged"

**Cause**: Events are either filtered out or no workload generating high dwell

**Solution**:
```bash
# 1. Generate high-dwell workload
cd test
go run workload_generator.go

# 2. Watch logs in real-time
sudo tail -f /var/log/syslog | grep "High dwell"

# 3. Check filtering is working correctly
# Should see: events < 0.1s not logged
# Should see: events < 2s not logged
# Should see: events > 2s logged with "⏱️ High dwell:"

# 4. If still nothing, try simulation
./bin/dwell-fiber-daemon --simulate
```

---

### Problem: "Metrics endpoint not responding"

**Cause**: Port already in use or metrics server not started

**Solution**:
```bash
# 1. Check port
netstat -tuln | grep 9090

# 2. Kill existing daemon
pkill dwell-fiber-daemon
sleep 1

# 3. Try different port
./bin/dwell-fiber-daemon --port=9091
# Then check: curl http://localhost:9091/metrics

# 4. Check if metrics server started in logs
tail -20 /tmp/dwell-fiber-test/daemon.log | grep -i metric

# 5. Restart daemon
sudo systemctl restart dwell-fiber-daemon  # if installed as service
```

---

### Problem: "Firefox dashboard shows 0.0 for all values"

**Cause**: No events processed yet, or simulation showing initial state

**Solution**:
```bash
# 1. Run workload generator
cd test
go run workload_generator.go

# 2. Wait a few seconds
sleep 5

# 3. Refresh Firefox
# Press F5 or Ctrl+R

# 4. Check if metrics are updating
watch 'curl -s http://localhost:9090/metrics'

# 5. If still zeros, check if in simulation mode
ps aux | grep dwell-fiber
# Look for "--simulate" flag

# 6. If not in simulation, try it
./bin/dwell-fiber-daemon --simulate
```

---

### Problem: "Dashboard shows status 🟢 NORMAL even with high dwell"

**Cause**: Events < 2s are filtered (correctly), so average dwell stays low

**Solution**:
```bash
# 1. Generate MORE high-dwell operations
cd test
for i in {1..5}; do
    go run workload_generator.go -critical=5
    sleep 2
done

# 2. Check individual events (not just average)
grep "High dwell" /var/log/syslog

# 3. The status is based on AVERAGE dwell (rolling window)
# If you have many < 2s events mixed in, average stays low
# This is correct filtering behavior

# 4. To see enforcement triggers, watch for:
# "🐌 [DRY-RUN] Would throttle" (at 7s+)
# "💀 [DRY-RUN] Would kill" (at 15s+)
```

---

## Enforcement Issues

### Problem: "Enforcement not triggering (no 🐌 or 💀 symbols)"

**Cause**: Thresholds not being exceeded, or enforcement disabled

**Solution**:
```bash
# 1. Check thresholds in code
grep -n "ThrottleThreshold\|KillThreshold" daemon/controller.go

# 2. Check if enforcement is enabled
grep -n "enfConfig.Enabled" daemon/controller.go
# Should see: enfConfig.Enabled = true

# 3. Check if enforcement is in dry-run mode
grep -n "DRY-RUN" daemon/enforcer.go
# This is normal - shows what WOULD happen

# 4. Generate events that exceed thresholds
# Throttle: > 5 seconds
# Kill: > 15 seconds
cd test
go run workload_generator.go -critical=10  # 9s > 5s threshold
```

---

### Problem: "Enforcement running but not actually throttling/killing"

**Cause**: Likely in dry-run mode (which is safe default)

**Solution**:
```bash
# 1. Check the logs - should say "DRY-RUN"
# If you see "[DRY-RUN]", enforcement is in safe test mode

# 2. To enable real enforcement (DANGEROUS):
# Edit daemon/controller.go line 36:
# Change: enfConfig.Enabled = true  (this is dry-run)
# To: enfConfig.Enabled = true, enfConfig.DryRun = false

# WARNING: This will actually throttle/kill processes!

# 3. For now, just validate the dry-run messages appear:
grep "DRY-RUN" /var/log/syslog
```

---

## Metric/Dashboard Issues

### Problem: "Metrics show very small prices (< 0.01)"

**Cause**: Few high-dwell events or averaging with low-dwell events

**Solution**:
```bash
# 1. Generate continuous high-dwell load
cd test
while true; do
    go run workload_generator.go -high=1 -critical=1
    sleep 2
done

# 2. Monitor price increase
watch 'curl -s http://localhost:9090/metrics | grep dwell_fiber_price'

# Expected: 0.1 → 0.3 → 0.5 → 0.7+ as dwell accumulates

# 3. Verify ADMM formula is working
# p(t+1) = max(0, p(t) + α*(dwell - budget))
# With α=0.5, budget=5:
# If dwell=7: p += 0.5*(7-5) = 0.5*2 = 1.0
# So each 7s event adds 1.0 to price
```

---

### Problem: "Dashboard auto-refresh not working"

**Cause**: Browser caching or JavaScript issue

**Solution**:
```bash
# 1. Hard refresh
Ctrl+Shift+R  (or Cmd+Shift+R on Mac)

# 2. Open developer console
F12

# 3. Check for errors
Look in Console tab

# 4. Try different browser
firefox http://localhost:9090
chrome http://localhost:9090

# 5. Check HTTP header
curl -v http://localhost:9090 | grep "refresh\|cache"

# 6. Manual refresh approach
while true; do
    curl http://localhost:9090/metrics | grep dwell
    sleep 1
done
```

---

## Workload Generator Issues

### Problem: "Workload generator runs but no files created"

**Cause**: Directory permissions or path issue

**Solution**:
```bash
# 1. Check output directory exists
ls -la /tmp/dwell-fiber-workload/

# 2. Create if missing
mkdir -p /tmp/dwell-fiber-workload

# 3. Check permissions
chmod 755 /tmp/dwell-fiber-workload

# 4. Run again
cd test && go run workload_generator.go

# 5. Verify files created
ls -lh /tmp/dwell-fiber-workload/*
```

---

### Problem: "Workload generator runs but dwell times are short"

**Cause**: Python sleep timing or buffering issue

**Solution**:
```bash
# 1. Check workload source
cat test/workload_generator.go | grep -A5 "time.Sleep"

# 2. The workload SHOULD keep files open
# Check for: time.Sleep(duration)

# 3. Verify on system
# In terminal while workload running:
lsof | grep workload_generator | head -5
# Should show files open for specified duration

# 4. If times are short, check:
strace -e open,close,write go run test/workload_generator.go 2>&1 | head -50
```

---

## Integration Test Issues

### Problem: "./test/test-suite.sh: Permission denied"

**Solution**:
```bash
chmod +x test/test-suite.sh
./test/test-suite.sh
```

---

### Problem: "test-suite.sh fails at workload generation"

**Cause**: Python not installed or path wrong

**Solution**:
```bash
# 1. Check Python
python3 --version

# 2. Install if needed
sudo apt-get install python3

# 3. Try standalone workload
cd test && go run workload_generator.go

# 4. If that works, issue is in test-suite.sh
# Check the script uses same workload method
```

---

## Performance Issues

### Problem: "Daemon uses high CPU"

**Cause**: Tight event processing loop or high syscall volume

**Solution**:
```bash
# 1. Check event rate
grep "📥 Received event" /tmp/dwell-fiber-test/daemon.log | wc -l
# Should be manageable (< 100/sec in idle)

# 2. Monitor CPU
top -p $(pgrep dwell-fiber-daemon)
# Expected: < 5% in idle, < 20% under load

# 3. If high, check for:
# - Event loop without yielding
# - No backpressure on ring buffer
# - Enforcement operations taking too long

# 4. Try simulation mode to isolate BPF
./bin/dwell-fiber-daemon --simulate
```

---

### Problem: "Daemon uses high memory"

**Cause**: Event history buffer growing unbounded

**Solution**:
```bash
# 1. Check memory usage
ps aux | grep dwell-fiber-daemon | grep -v grep

# Expected: < 100MB after 1 minute

# 2. Check cleanup is running
grep "Cleanup" /tmp/dwell-fiber-test/daemon.log

# 3. Verify recentDwells buffer limited
grep -n "maxRecent\|len(c.recentDwells)" daemon/controller.go
# Should see: maxRecent = 100

# 4. If memory still growing:
# - Check enforcer cleanup
# - Verify old entries removed from dwellMap
```

---

## Testing Workflow Issues

### Problem: Can't reproduce the test scenarios

**Solution**:
```bash
# 1. Use daemon/test_suite.go for programmatic tests
# Create a test file:
cat > test_runner.go << 'EOF'
package main
import (
    "dwell-fiber/daemon"
    "fmt"
)

func main() {
    controller := NewController(0.5, 5.0)
    scenarios := GenerateTestScenarios()
    for _, scenario := range scenarios {
        SimulateScenario(controller, scenario)
    }
}
EOF

# 2. Run it
go run test_runner.go

# 3. Or use test-suite.sh for integration
./test/test-suite.sh
```

---

## Getting Help

If you're still stuck:

1. **Collect diagnostics:**
```bash
uname -a
go version
clang --version
coq -v
dmesg | grep -i "dwell\|ebpf" | tail -20
tail -100 /tmp/dwell-fiber-test/daemon.log
curl http://localhost:9090/metrics
```

2. **Check existing issues:**
```bash
grep -r "ERROR\|WARN" . --include="*.go"
```

3. **Try simplest test first:**
```bash
make clean all
./bin/dwell-fiber-daemon --simulate
# This should always work
```

4. **If still broken:**
- Create a GitHub issue
- Include the diagnostics above
- Describe what you're trying to do
- Include exact error message

---

## Quick Sanity Checks

Run these in order:

```bash
# 1. Build works
make daemon && echo "✓ Build OK"

# 2. Daemon starts
timeout 2 ./bin/dwell-fiber-daemon --simulate && echo "✓ Daemon starts"

# 3. Metrics endpoint works
sleep 1 && curl -s http://localhost:9090/metrics | head && echo "✓ Metrics OK"

# 4. BPF loads (with real mode)
sudo timeout 2 ./bin/dwell-fiber-daemon && echo "✓ BPF loads"

# 5. Workload runs
cd test && timeout 5 go run workload_generator.go && echo "✓ Workload OK"
```

If all 5 pass, the system is working correctly!
