## 📄 Dwell-Fiber V3.1: Future Work Rationale Dossier

This document outlines the strategic rationale and necessary research paths for the next major iteration of the Dwell-Fiber system, **V3.1: File-Type Adaptive Pricing**. The pivot is motivated by emerging ransomware evasion techniques that target the static thresholds of the current V3.0 **Trust Classification Module (TCM)**.

---

## 1. 🔍 Motivation: Evasion of Static V3.0 Thresholds

The V3.0 architecture successfully mitigated the Dwell-Time flaw by introducing the **Weighted I/O Pressure ($\text{WIP}$)** metric. However, its effectiveness is now bounded by its reliance on **global, static classification thresholds** (e.g., $\text{UFM} > 1,000 \text{ files/s}$) for all file types.

### A. The Next Generation Attack Vector
An advanced ransomware variant could:
1.  **Dwell Just Below the Line:** Modulate its Unique Files Modified ($\text{UFM}$) rate to stay just below the lowest $\text{T2: Untrusted}$ threshold, thus receiving the lowest dual price ($\pi$).
2.  **Exploit File-Type Variance:** Exploit that a low $\text{UFM}$ rate (e.g., $500 \text{ files/s}$) is **suspicious** for executables but potentially **normal** for large media files. V3.0 global thresholds treat these equally.

### B. The Research Mandate
V3.1 must evolve the TCM from static thresholds to a **context-aware, adaptive pricing oracle** that incorporates file metadata to prevent evasive behavior.

---

## 2. 🔬 V3.1 Core Innovation: File-Type Adaptive Pricing (FTAP)

The core feature of V3.1 is the **File-Type Adaptive Pricing (FTAP)** mechanism: the eBPF monitor must expose file type information and the daemon must apply a calibrated budget/weight matrix.

### A. The Adaptive Threshold Matrix
Replace single budget with a matrix:

$$\text{Budget}_{\text{WIP}}^{\text{Tier,FileType}}$$

TCM must lookup both Tier ID and file extension/magic to pick ω and budget.

Example mapping (illustrative):

| File Type Category | Base Sensitivity | Pricing Strategy (ω) | Example UFM Threshold |
|---|---:|---|---:|
| Executables/System (`.exe`, `.dll`) | HIGH | Price UFM aggressively (ω₂ ≫ ω₁) | 50 files/s |
| Documents/Code (`.docx`, `.py`) | MEDIUM | Price UFM moderately (ω₁ ≈ ω₂) | 500 files/s |
| Large Media/Archives (`.mp4`, `.vhd`) | LOW | Price TBW more (ω₁ > ω₂) | 5,000 files/s |

### B. eBPF Integration Requirement
Refactor eBPF (`bpf/dwell_monitor.bpf.c`) to capture file-type metadata (extension or magic) and emit it in the event struct.

---

## 3. 🛠️ Future Work Deliverables

| Task | Component | Description |
|---:|---|---|
| FTAP Implementation | daemon/dwell_user.go | Refactor TCM to accept file type and lookup Budget/Weight parameters |
| eBPF Metadata Exposure | bpf/dwell_monitor.bpf.c | Update io_event struct to include file-type/extension metadata |
| Continuous Calibration | metrics/ & Research | Empirically derive FTAP matrix and update thresholds continuously |
| Formal Re-Verification | coq/dwell_stable.v | Prove stability under a matrix of discontinuous weight switches |

---

## 4. 📈 Research & Validation Plan

1. Add file-type capture in a low-risk simulation mode (no enforcement).
2. Collect TBW/UFM per file-type across benign workloads (backups, builds, media jobs).
3. Derive per-file-type thresholds minimizing false positives/negatives.
4. Implement FTAP matrix and evaluate against adaptive ransomware simulations.
5. Update Coq proof obligations to account for file-type conditioned weight switching.

---

## 5. 🔒 Security Rationale

FTAP raises the attack cost by:
- Denying a single global evasion vector (tuning UFM to stay below one threshold).
- Forcing attackers to tailor behavior per file type, increasing complexity and economic cost.
- Allowing conservative pricing for high-entropy file types (reducing false positives).

---

## 6. ✅ Success Criteria

- Reliable file-type metadata available in user-space events.
- TCM uses file-type budgets to raise detection rate against adaptive samples by ≥ 90% while keeping false positives < 1% on benign workloads.
- Coq proof extended to certify bounded price drift under FTAP switching model.

---

## 7. Next Steps & Estimate

- Prototype eBPF metadata emission: 4–6h
- Gather representative workload metrics: 8–16h
- Implement FTAP lookup and integrate into controller: 6–10h
- Formal proof extension and verification: 20–40h
- Total: ~38–72h (research-heavy; depends on dataset availability)

---

**Prepared:** Dwell-Fiber research team  
**Purpose:** Rationale & roadmap for V3.1 (FTAP) integration and research.
