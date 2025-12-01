# Coq Installation Guide for Dwell-Fiber

## Overview

This guide provides instructions for installing Coq and required dependencies to compile and verify the Dwell-Fiber formal proofs.

## System Requirements

- **Operating System**: Linux, macOS, or Windows
- **RAM**: Minimum 4GB (8GB recommended for proof compilation)
- **Disk Space**: 2GB for Coq installation + 500MB for dependencies

## Installation Methods

### Option 1: Package Manager (Recommended)

#### Ubuntu/Debian
```bash
sudo apt-get update
sudo apt-get install coq
```

#### macOS (Homebrew)
```bash
brew install coq
```

#### Windows
Download the installer from: https://coq.inria.fr/download

### Option 2: OPAM (OCaml Package Manager)

This method provides the most up-to-date Coq version and better dependency management.

```bash
# Install OPAM first (if not already installed)
# Ubuntu/Debian:
sudo apt-get install opam

# macOS:
brew install opam

# Initialize OPAM
opam init --bare
eval $(opam env)

# Install Coq
opam install coq

# Verify installation
coqc --version
```

### Option 3: From Source

For developers who need the latest features or want to contribute to Coq:

```bash
# Install dependencies first
# Ubuntu/Debian:
sudo apt-get install build-essential ocaml ocaml-native-compilers camlp5

# Clone Coq repository
git clone https://github.com/coq/coq.git
cd coq

# Configure and build
./configure -prefix /usr/local
make -j4
sudo make install
```

## Verifying Installation

After installation, verify that Coq is properly installed:

```bash
# Check Coq version
coqc --version

# Should output something like:
# The Coq Proof Assistant, version 8.15.2
# compiled with OCaml 4.13.1

# Test basic compilation
echo "Print nat." > test.v
coqc test.v
# Should create test.vo and test.glob files
rm test.v test.vo test.glob
```

## Required Coq Libraries

The Dwell-Fiber proofs use standard Coq libraries that come with the base installation:

- `Reals` - Real number arithmetic
- `ZArith` - Integer arithmetic
- `Lia` - Linear integer arithmetic solver
- `Lra` - Linear real arithmetic solver
- `List` - List data structures
- `Max` - Maximum/minimum functions
- `RIneq` - Real number inequalities

These libraries are included with the standard Coq distribution.

## Troubleshooting

### Issue: `coqc: command not found`

**Solution**: Coq is not in your PATH. Add it manually:

```bash
# Find where Coq is installed
find /usr -name coqc 2>/dev/null

# Add to PATH (example for /usr/local/bin)
export PATH=$PATH:/usr/local/bin
# Add this line to your ~/.bashrc or ~/.zshrc
```

### Issue: Missing dependencies during compilation

**Solution**: Install required OCaml packages:

```bash
opam install ocamlfind camlp5
```

### Issue: Out of memory during proof compilation

**Solution**: Increase OCaml heap size:

```bash
export OCAMLRUNPARAM="s=256M"
```

### Issue: Windows-specific path problems

**Solution**: Use Windows Subsystem for Linux (WSL) or ensure paths use forward slashes:

```bash
# Instead of:
coqc -R C:\Users\name\project DwellFiber file.v

# Use:
coqc -R /c/Users/name/project DwellFiber file.v
```

## Continuous Integration Setup

For automated verification in CI/CD pipelines:

### GitHub Actions
```yaml
name: Verify Coq Proofs

on: [push, pull_request]

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: coq-community/docker-coq-action@v1
        with:
          coq_version: '8.15.2'
          custom_script: |
            cd coq
            make
```

### GitLab CI
```yaml
verify:coq:
  image: coqorg/coq:8.15.2
  script:
    - cd coq
    - make
```

## Next Steps

Once Coq is installed:

1. **Verify Dwell-Fiber proofs**:
   ```bash
   cd dwell-fiber-1
   make verify
   ```

2. **Run the verification script**:
   ```bash
   chmod +x verify-coq-installation.sh
   ./verify-coq-installation.sh
   ```

3. **Check proof status**:
   ```bash
   cd coq
   make
   ```

## Support

If you encounter issues:

1. Check the [Coq FAQ](https://github.com/coq/coq/wiki/FAQ)
2. Search [Coq GitHub issues](https://github.com/coq/coq/issues)
3. Ask on [Coq Discourse](https://coq.discourse.group/)
4. Review [Coq documentation](https://coq.inria.fr/documentation)

## Version Compatibility

The Dwell-Fiber proofs are compatible with:
- **Coq**: 8.13.0 or later (tested with 8.15.2)
- **OCaml**: 4.10.0 or later

Older versions may work but are not officially supported.