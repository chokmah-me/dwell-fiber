## 📄 Dwell-Fiber V3.1: Future Work Rationale Dossier

This document outlines the strategic rationale and necessary research paths for the next major iteration of the Dwell-Fiber system, **V3.1: File-Type Adaptive Pricing**. The pivot is motivated by emerging ransomware evasion techniques that target the static thresholds of the current V3.0 **Trust Classification Module (TCM)**.

---

## 1. 🔍 Motivation: Evasion of Static V3.0 Thresholds

The V3.0 architecture successfully mitigated the Dwell-Time flaw by introducing the **Weighted I/O Pressure ($\text{WIP}$)** metric. However, its effectiveness is now bounded by its reliance on **global, static classification thresholds** (e.g., $\text{UFM} > 1,000 \text{ files/s}$) for all file types.

### A. The Next Generation Attack Vector
The V3.1 goal is to evolve the TCM from a single, static classification function to a **context-aware, adaptive pricing oracle** that incorporates file metadata to prevent evasive behavior.

---

## 2. 🔬 V3.1 Core Innovation: File-Type Adaptive Pricing (FTAP)

The core feature of V3.1 is the **File-Type Adaptive Pricing (FTAP)** mechanism, which requires the eBPF monitor to expose file type information and the Daemon to host a matrix of calibrated thresholds.

### A. The Adaptive Threshold Matrix
The single $\text{budget}_{\text{WIP, Tier}}$ used in V3.0 must be replaced by a matrix of budgets, $\text{Budget}_{\text{WIP, Tier}}^{\text{File Type}}$, derived from empirical analysis.

### B. eBPF Integration Requirement
Refactor the eBPF monitor to obtain and expose the file's **MIME type** or **extension** and include it in the `io_event` struct sent to the daemon.

---

## 3. 🛠️ Future Work Deliverables

| Task | Component | Description |
| :--- | :--- | :--- |
| **FTAP Implementation** | `daemon/dwell_user.go` | Refactor the TCM to accept the file type and use a look-up table for `Budget/Weight` parameters. |
| **eBPF Metadata Exposure** | `bpf/dwell_monitor.bpf.c` | Update the `io_event` struct and kernel logic to capture and stream file type/extension metadata alongside $\text{TBW}$ and $\text{UFM}$. |
| **Continuous Calibration** | `metrics/` & Research | Systematically test V3.1 against emerging ransomware families to empirically derive and update the **FTAP matrix**. |
| **Formal Re-Verification** | `coq/dwell_stable.v` | Update the stability proof to certify the system remains convergent and bounded under a **matrix of discontinuous weight switches**. |

This V3.1 framework moves Dwell-Fiber to a robust context-aware pricing engine.
