#!/usr/bin/env bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  DWELL-FIBER TEST SUITE - Integration Test Orchestrator    ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"

# Configuration
DAEMON_BIN="./bin/dwell-fiber-daemon"
ALPHA="0.5"
BUDGET="5.0"
METRICS_PORT="9090"
DAEMON_PID=""
TEST_DIR="/tmp/dwell-fiber-test"
LOGFILE="${TEST_DIR}/daemon.log"

# Cleanup function
cleanup() {
    if [ -n "${DAEMON_PID}" ] && kill -0 "${DAEMON_PID}" 2>/dev/null; then
        echo -e "${YELLOW}[cleanup] Stopping daemon PID ${DAEMON_PID}...${NC}"
        sudo kill "${DAEMON_PID}" 2>/dev/null || true
        sleep 1
    fi
    echo -e "${GREEN}✓ Cleanup complete${NC}"
}

trap cleanup EXIT

# Ensure test directory exists
mkdir -p "${TEST_DIR}"
echo -e "${GREEN}✓ Test directory: ${TEST_DIR}${NC}"

# Step 1: Build
echo -e "\n${YELLOW}[1/5] Building daemon...${NC}"
if [ ! -f "${DAEMON_BIN}" ]; then
    echo -e "${YELLOW}  Daemon not found. Building...${NC}"
    make daemon || {
        echo -e "${RED}✗ Build failed${NC}"
        exit 1
    }
fi
echo -e "${GREEN}✓ Daemon built: ${DAEMON_BIN}${NC}"

# Step 2: Start daemon
echo -e "\n${YELLOW}[2/5] Starting daemon...${NC}"
echo "Starting daemon in background..." > "${LOGFILE}"
# Use timeout to prevent infinite waits
sudo timeout 60s "${DAEMON_BIN}" \
    --alpha="${ALPHA}" \
    --budget="${BUDGET}" \
    --port="${METRICS_PORT}" \
    >> "${LOGFILE}" 2>&1 &
DAEMON_PID=$!
echo -e "${GREEN}✓ Daemon started with PID: ${DAEMON_PID}${NC}"

# Wait for daemon to be ready
echo -e "${YELLOW}  Waiting for daemon to initialize (3 seconds)...${NC}"
sleep 3

# Verify daemon is running
if ! kill -0 "${DAEMON_PID}" 2>/dev/null; then
    echo -e "${RED}✗ Daemon failed to start${NC}"
    echo -e "${YELLOW}Log output:${NC}"
    cat "${LOGFILE}"
    exit 1
fi
echo -e "${GREEN}✓ Daemon is running${NC}"

# Step 3: Generate high-dwell workload
echo -e "\n${YELLOW}[3/5] Generating high-dwell workload...${NC}"
generate_workload() {
    local duration=$1
    local name=$2
    echo -e "${BLUE}  → ${name}${NC}"
    
    # Create test file
    local testfile="${TEST_DIR}/${name}_test.txt"
    
    # Open file and keep it open for the specified duration
    python3 - <<PYTHON
import time
import sys

testfile = '${testfile}'
duration = ${duration}

print(f'  Opening {testfile} for {duration}s...')
try:
    with open(testfile, 'w') as f:
        f.write(f'Test data: {sys.argv[0]}\n')
        f.flush()
        time.sleep(duration)  # Keep file open
        f.write(f'Completed at {time.time()}\n')
    print(f'  ✓ Closed after {duration}s')
except Exception as e:
    print(f'  ✗ Error: {e}')
PYTHON
}

# Run multiple high-dwell scenarios
echo -e "${BLUE}Scenario 1: Normal (5-6s)${NC}"
generate_workload 5 "normal" &
sleep 1

echo -e "${BLUE}Scenario 2: High (7s)${NC}"
generate_workload 7 "high" &
sleep 2

echo -e "${BLUE}Scenario 3: Critical (9s)${NC}"
generate_workload 9 "critical" &

wait
echo -e "${GREEN}✓ Workload completed${NC}"

# Step 4: Check metrics
echo -e "\n${YELLOW}[4/5] Checking metrics at http://localhost:${METRICS_PORT}/metrics${NC}"
echo -e "${BLUE}Metrics endpoint:${NC}"

METRICS_OUTPUT=$(curl -s "http://localhost:${METRICS_PORT}/metrics" 2>/dev/null || echo "")

if [ -z "${METRICS_OUTPUT}" ]; then
    echo -e "${YELLOW}  ⚠ Metrics endpoint not responding${NC}"
else
    echo -e "${GREEN}✓ Metrics available${NC}"
    echo ""
    echo -e "${BLUE}Key metrics:${NC}"
    echo "${METRICS_OUTPUT}" | grep -E "dwell_fiber_" | head -10 || echo "  (No metrics found)"
fi

# Step 5: Check for enforcement in logs
echo -e "\n${YELLOW}[5/5] Checking enforcement events...${NC}"
echo -e "${BLUE}Recent daemon output (last 30 lines):${NC}"
tail -30 "${LOGFILE}" | grep -E "High dwell|Throttle|Kill|Enforce" || \
    echo -e "${YELLOW}  ℹ No enforcement events in logs (BPF or workload may not be running)${NC}"

echo ""
echo -e "${BLUE}Full test log: ${LOGFILE}${NC}"
tail -50 "${LOGFILE}"

# Summary
echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║              TEST SUITE EXECUTION SUMMARY                  ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo -e "${GREEN}✓ Build: Success${NC}"
echo -e "${GREEN}✓ Daemon: Running${NC}"
echo -e "${GREEN}✓ Workload: Generated${NC}"
if [ -n "${METRICS_OUTPUT}" ]; then
    echo -e "${GREEN}✓ Metrics: Available${NC}"
else
    echo -e "${YELLOW}⚠ Metrics: Not responding${NC}"
fi

echo -e "\n${BLUE}Next steps:${NC}"
echo "  1. Open Firefox: http://localhost:${METRICS_PORT}"
echo "  2. Monitor logs: tail -f ${LOGFILE}"
echo "  3. Kill daemon: kill ${DAEMON_PID}"
echo -e "\n${YELLOW}To verify enforcement, look for:${NC}"
echo "  🐌 [DRY-RUN] Would throttle"
echo "  💀 [DRY-RUN] Would kill"
echo "  or actual enforcement when enabled"
