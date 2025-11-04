#!/bin/bash
# Quick VM setup script for dwell-fiber
# Run this on your Ubuntu 25.10 VM: bash vm-setup.sh

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║    Dwell-Fiber VM Setup Script                            ║"
echo "║    Ubuntu 25.10 - Complete Initialization                 ║"
echo "╚════════════════════════════════════════════════════════════╝"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Step 1: Update system
echo -e "\n${YELLOW}[1/6] Updating system packages...${NC}"
sudo apt-get update -qq
sudo apt-get upgrade -y -qq
echo -e "${GREEN}✓ System updated${NC}"

# Step 2: Install dependencies
echo -e "\n${YELLOW}[2/6] Installing build dependencies...${NC}"
sudo apt-get install -y -qq \
    clang llvm libbpf-dev \
    linux-headers-$(uname -r) \
    golang-go coq make git \
    netcat-openbsd python3
echo -e "${GREEN}✓ Dependencies installed${NC}"

# Step 3: Fix asm symlink (CRITICAL for Ubuntu 25.10)
echo -e "\n${YELLOW}[3/6] Fixing asm symlink (Ubuntu 25.10 critical fix)...${NC}"
if [ ! -L /usr/include/asm ]; then
    sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm
    echo -e "${GREEN}✓ Symlink created${NC}"
else
    echo -e "${GREEN}✓ Symlink already exists${NC}"
fi

# Verify
if ls -la /usr/include/asm > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Verified: /usr/include/asm accessible${NC}"
else
    echo -e "${RED}✗ ERROR: asm symlink not working${NC}"
    exit 1
fi

# Step 4: Clone or update repo
echo -e "\n${YELLOW}[4/6] Cloning/updating dwell-fiber repository...${NC}"
if [ ! -d dwell-fiber ]; then
    git clone https://github.com/dyb5784/dwell-fiber.git
    cd dwell-fiber
    echo -e "${GREEN}✓ Repository cloned${NC}"
else
    cd dwell-fiber
    git fetch origin
    git pull origin main
    echo -e "${GREEN}✓ Repository updated${NC}"
fi

# Step 5: Build
echo -e "\n${YELLOW}[5/6] Building dwell-fiber (this may take a moment)...${NC}"
make clean all
if [ -f bin/dwell-fiber-daemon ]; then
    echo -e "${GREEN}✓ Build successful${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

# Step 6: Verify build
echo -e "\n${YELLOW}[6/6] Verifying build artifacts...${NC}"
file bin/dwell-fiber-daemon | grep -q "ELF" && echo -e "${GREEN}✓ Daemon binary OK${NC}"
[ -f bpf/dwell_monitor.bpf.o ] && echo -e "${GREEN}✓ BPF object OK${NC}"
[ -f coq/dwell_stable.vo ] && echo -e "${GREEN}✓ Coq proofs OK${NC}"

# Success!
echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║           SETUP COMPLETE! Ready for testing               ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"

echo -e "\n${YELLOW}Next steps:${NC}"
echo -e "  1. Test simulation mode (no root):"
echo -e "     ${YELLOW}./bin/dwell-fiber-daemon --simulate${NC}"
echo ""
echo -e "  2. Open dashboard in Firefox:"
echo -e "     ${YELLOW}http://localhost:9090${NC}"
echo ""
echo -e "  3. Generate workload (new terminal):"
echo -e "     ${YELLOW}cd test && go run workload_generator.go${NC}"
echo ""
echo -e "  4. For complete guide, see:"
echo -e "     ${YELLOW}cat VM_SETUP_GUIDE.md${NC}"
echo ""
echo -e "  5. For full integration test:"
echo -e "     ${YELLOW}./test/test-suite.sh${NC}"
echo ""
echo -e "${GREEN}Happy testing! 🛡️${NC}"
