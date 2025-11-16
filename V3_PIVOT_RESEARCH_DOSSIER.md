## 📄 Dwell-Fiber V3.0: Pivot Research Dossier

This document provides a formal explanation of the research and empirical analysis that necessitated the strategic pivot from the **V2.x (Dwell-Time)** architecture to the **V3.0 (Weighted I/O Pressure, or WIP)** architecture.

***

## 1. 🔍 Executive Summary: The Failure of the Dwell Metric

The Dwell-Fiber project's security guarantee was founded on a core tenet of **Network-Utility-Maximisation (NUM)**: *price the scarce resource until consumption hits the budget.*

In V2.x, the scarce resource was **File Access Time** (Dwell Time). The assumption was that ransomware needed a long, sustained file access session to encrypt data.

The pivot to V3.0 was mandated by the empirical failure of this assumption against modern, high-velocity ransomware variants (e.g., **LockBit 3.0** and subsequent copycats). These variants bypass the Dwell-Time constraint by utilizing **intermittent/partial encryption** methods.

The new mandate was clear: the system must price **I/O Throughput and Scattering Rate**, not just session length.

---

## 2. 📉 V2.x Architecture: The Dwell-Time Flaw

### A. V2.x Core Mechanism
The original system, as detailed in `architecture.md`, used an **ADMM (Alternating Direction Method of Multipliers)** approach to dynamically price file access based on the duration a process held a file open.

* **Metric (V2.x):** **Dwell Time** ($\text{dwell}_{i}(t)$) - Measured time between `sys_openat` and `sys_close` per PID/inode pair.
* **Budget:** $\text{budget}_{\text{dwell}} = 5.0$ seconds.
* **Control Loop:**
    $$\pi_{i}(t+1) = \max\left(0, \pi_{i}(t) + \alpha \times \left(\text{dwell}_{i}(t) - \text{budget}\right)\right)$$

### B. The Empirical Failure (The "LockBit Problem")
Research into modern ransomware attack techniques revealed a critical vulnerability in this V2.x model:

1.  **Intermittent Access:** Ransomware no longer opens a target file, encrypts the entire thing, and then closes it. Instead, it opens the file, encrypts a small *chunk* (e.g., the first 1MB or small, random blocks), and immediately closes it.
2.  **Sub-second Dwell:** This access pattern results in $\text{dwell}_{i}(t)$ being **consistently less than 100 milliseconds** (far below the $5.0\text{s}$ budget).
3.  **Pricing Bypass:** Since the measured $\text{dwell}$ never exceeded the budget, the term $(\text{dwell} - \text{budget})$ remained negative, causing the dual price $\pi_i$ to never rise. The ransomware was effectively granted **unlimited, zero-cost I/O access**, completely invalidating the security guarantee proven in `coq/dwell_stable.v`.

---

## 3. 🔬 Pivot Rationale: Pricing Resource Consumption Rate

The research mandate for V3.0 was to find a metric that captures the **high-speed, scattered nature** of the I/O activity, which is the true signature of an encryption attack.

The solution involved returning to first-principles: **NUM requires pricing the true coupling constraint**. The constraint was not *time per file*, but the system's ability to handle **high I/O bandwidth and metadata modification pressure** without instability.

### A. Identifying Key Indicators
Analysis of ransomware telemetry (LockBit, Conti, etc.) versus benign high-I/O processes (e.g., system backups, development builds) identified two orthogonal features:

1.  **Total Bytes Written ($\text{TBW}$):** Volume indicator. High in *both* ransomware and benign backups.
2.  **Unique Files Modified ($\text{UFM}$):** Scattergun indicator. Extremely high in ransomware (thousands of files modified per second), but also high in complex compilation/build processes.

### B. The Adaptive Weighting Requirement
Since $\text{TBW}$ and $\text{UFM}$ alone are insufficient to distinguish between a benign backup and an attack, the pivot required an **Adaptive Pricing Mechanism**—the core innovation of V3.0:

* **TCM (Trust Classification Module):** A userspace module that classifies the process into a **Tier** ($T1, T1.5, T2$) based on the *ratio* of $\text{TBW}$ to $\text{UFM}$.
* **Dynamic Weights ($\omega$):** The ADMM dual ascent must now use a **Weighted Metric** where the weights ($\omega_1, \omega_2$) are dynamically adjusted based on the process's Tier.

This adaptive pricing is the theoretical breakthrough that restored the provable convergence guarantee, albeit with the need for new, complex formal proofs to certify the stability under non-smooth, dynamic weight switching.

---

## 4. ⚙️ V3.0 Solution: Weighted I/O Pressure ($\text{WIP}$)

The V3.0 architecture replaces Dwell Time with the **Weighted I/O Pressure ($\text{WIP}$)** metric, measured over a fixed **$1.0\text{s}$ sampling window ($\Delta t$)**.

* **New Metric (V3.0):**
    $$\text{WIP}_{i}(t) = (\omega_1 \cdot \text{TBW}_{i}) + (\omega_2 \cdot \text{UFM}_{i})$$

* **New Control Loop:** The ADMM price update is now coupled to the TCM classification, which determines the budget for that Tier.
    $$\pi_{i}(t+1) = \max\left(0, \pi_{i}(t) + \alpha \times \left(\text{WIP}_{i}(t) - \text{budget}_{\text{WIP, Tier}}\right)\right)$$

This pivot shifts the security enforcement from a **temporal constraint** (how long a file is held) to a **rate-based, economic constraint** (how much the system will tolerate being written/scattered per second before pricing the process out of existence). This addresses the root cause of the V2.x failure while maintaining the **provable stability** mandated by the NUM/ADMM framework.
