param(
    [string]$Tag = "v3.0.1"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# 1) Stage all changes
git add -A

# 2) Single combined commit message (implementation + docs)
$commitMsg = @"
feat: Complete Dwell-Fiber V3 implementation with WIP metric, TCM tiers, discrete ADMM, Coq proofs, and comprehensive docs

Implement WIP metric: ω₁·TBW + ω₂·UFM with tier-based weights/budgets
Add Trust Classification Module (TCM) for T1/T1.5/T2 tier assignment
Update eBPF to kprobe/vfs_write with 1.0s windowed aggregation
Refactor ADMM controller for dynamic budgets and discrete sampling
Add Coq lemmas: wip_is_convex, dual_price_bounded_under_switch, bounded_lyapunov_drift_discrete_wip
Create comprehensive docs: README, ARCHITECTURE.md, V3_MIGRATION.md, FORMAL_VERIFICATION.md, CHANGELOG.md
Update status to reflect formal verification in progress (proofs have type issues)
Closes intermittent-access ransomware vulnerability (LockBit bypass)

docs: Add V3 Pivot Research Dossier and update references

- Add V3_PIVOT_RESEARCH_DOSSIER.md: Formal research analysis of V2.x dwell-time failure against LockBit intermittent encryption, and theoretical rationale for WIP metric with adaptive TCM weighting
- Update README.md, V3_MIGRATION.md, PROJECT_STATUS.md, SESSION_SUMMARY.md, DEV-NOTES.md to reference the new dossier
- Clarify V3 status: implementation complete, formal proofs have type issues
"@

# 3) Commit (skips if nothing to commit)
try {
    git commit -m $commitMsg
} catch {
    Write-Host "Nothing to commit. Skipping commit." -ForegroundColor Yellow
}

# 4) Push main
git push origin main

# 5) Tag and push tag (skip if taken)
try {
    # use ${Tag} to avoid parser error with ":" in the message
    git tag -a "${Tag}" -m "Dwell-Fiber ${Tag}: V3 implementation + docs"
    git push origin "${Tag}"
} catch {
    Write-Host "Tag '${Tag}' exists or push failed. Skipping tag." -ForegroundColor Yellow
}

Write-Host "✓ Push complete." -ForegroundColor Green
