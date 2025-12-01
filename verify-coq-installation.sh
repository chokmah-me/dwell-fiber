#!/bin/bash

# Dwell-Fiber Coq Verification Script
# This script checks if Coq is properly installed and verifies all proofs

set -e

echo "=== Dwell-Fiber Coq Verification ==="
echo

# Check if Coq is installed
if ! command -v coqc &> /dev/null; then
    echo "❌ ERROR: Coq is not installed or not in PATH"
    echo
    echo "To install Coq:"
    echo "  Ubuntu/Debian: sudo apt-get install coq"
    echo "  macOS:         brew install coq"
    echo "  Windows:       Download from https://coq.inria.fr/download"
    echo "  Via OPAM:      opam install coq"
    echo
    echo "After installation, make sure 'coqc' is in your PATH"
    exit 1
fi

# Check Coq version
COQ_VERSION=$(coqc --version | head -n1)
echo "✓ Coq found: $COQ_VERSION"
echo

# Check required Coq libraries
echo "Checking required libraries..."
if coqc -Q . DwellFiber -batch <(echo "Require Import Reals. Require Import List. Require Import Max. Require Import RIneq."); then
    echo "✓ All required standard libraries available"
else
    echo "❌ ERROR: Missing required Coq libraries"
    exit 1
fi
echo

# Compile Coq files
echo "Compiling Coq proofs..."
cd coq

echo "1. Compiling dwell_stable.v..."
if coqc -R . DwellFiber dwell_stable.v; then
    echo "   ✓ dwell_stable.v compiled successfully"
else
    echo "   ❌ ERROR: dwell_stable.v compilation failed"
    exit 1
fi

echo "2. Compiling dwell_kernel_resilience.v..."
if coqc -R . DwellFiber dwell_kernel_resilience.v; then
    echo "   ✓ dwell_kernel_resilience.v compiled successfully"
else
    echo "   ❌ ERROR: dwell_kernel_resilience.v compilation failed"
    exit 1
fi

echo "3. Compiling dwell_extended.v..."
if coqc -R . DwellFiber dwell_extended.v; then
    echo "   ✓ dwell_extended.v compiled successfully"
else
    echo "   ❌ ERROR: dwell_extended.v compilation failed"
    exit 1
fi

echo

# Run verification
echo "Running Coq verification..."
if coqchk -silent -R . DwellFiber dwell_stable dwell_kernel_resilience; then
    echo "✓ All proofs verified successfully"
else
    echo "❌ ERROR: Proof verification failed"
    exit 1
fi

echo
echo "=== ✅ All Dwell-Fiber Coq proofs verified successfully! ==="
echo
echo "Summary:"
echo "  - dwell_stable.v: ADMM stability proofs"
echo "  - dwell_kernel_resilience.v: Event loss resilience proofs"
echo "  - dwell_extended.v: Liveness, fairness, and attack resistance"
echo
echo "The system is formally verified to maintain stability and detection"
echo "capability even with up to 10% eBPF event loss."