#!/usr/bin/env bash
# v1.5.0 URL migration: dyb5784/dwell-fiber -> chokmah-me/dwell-fiber
#
# Idempotent. Run from repo root. Skips go.mod (intentional — see comment).
# Skips .git, vendor, bin.
#
# After running, review with `git diff` before committing.
set -euo pipefail

OLD_OWNER="dyb5784"
NEW_OWNER="chokmah-me"
OLD="${OLD_OWNER}/dwell-fiber"
NEW="${NEW_OWNER}/dwell-fiber"

# Files we touch: markdown, citation files, yaml (CI), shell scripts, plain text.
# Files we DO NOT touch:
#   - go.mod / *.go : module path is github.com/dyb5784/dwell-fiber and
#                     every Go import depends on it. Changing this breaks
#                     all internal imports for zero functional gain. Revisit
#                     only if you publish a paper that references the package path.
#   - .git/         : obvious
#   - vendor/, bin/ : build/dependency artifacts

echo "==> scanning for references to ${OLD}"
mapfile -t HITS < <(
    grep -rl "${OLD}" \
        --include='*.md' \
        --include='*.cff' \
        --include='*.bibtex' \
        --include='*.bib' \
        --include='*.yml' \
        --include='*.yaml' \
        --include='*.sh' \
        --include='*.txt' \
        --include='*.html' \
        --exclude-dir='.git' \
        --exclude-dir='vendor' \
        --exclude-dir='bin' \
        --exclude-dir='node_modules' \
        . 2>/dev/null || true
)

if [[ ${#HITS[@]} -eq 0 ]]; then
    echo "no references found; already migrated?"
    exit 0
fi

echo "files with references:"
printf '  %s\n' "${HITS[@]}"
echo

# In-place replace. BSD sed (mac) and GNU sed differ in -i syntax;
# this form works on both.
for f in "${HITS[@]}"; do
    if [[ "$(uname)" == "Darwin" ]]; then
        sed -i '' "s|${OLD}|${NEW}|g" "$f"
    else
        sed -i "s|${OLD}|${NEW}|g" "$f"
    fi
done

echo "==> done. Review with:"
echo "    git diff --stat"
echo "    git diff"
echo
echo "==> Files explicitly NOT touched (Go module path):"
grep -l "${OLD}" go.mod 2>/dev/null || echo "    (no go.mod hits)"
grep -rl "${OLD}" --include='*.go' --exclude-dir='.git' --exclude-dir='vendor' . 2>/dev/null \
    | sed 's/^/    /' || true
echo
echo "If you ever DO want to migrate the Go module path:"
echo "  1. Edit go.mod: module github.com/${NEW_OWNER}/dwell-fiber"
echo "  2. Update every Go import: find . -name '*.go' -exec sed -i \\"
echo "       's|github.com/${OLD_OWNER}/dwell-fiber|github.com/${NEW_OWNER}/dwell-fiber|g' {} +"
echo "  3. go mod tidy && go build ./..."
echo "  4. Test thoroughly. Bump major version, since this is a breaking change for any importer."
