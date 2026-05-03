#!/usr/bin/env bash
# Apply the v1.5.0 release plan to a clean working tree.
#
# Run from repo root:
#   bash apply_v1.5.0.sh /path/to/v1.5.0_artifacts
#
# Idempotent enough; assumes you started on a clean main and a v1.5.0 branch.
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "usage: $0 /path/to/v1.5.0_artifacts" >&2
    exit 1
fi

ART="$1"
[[ -d "$ART" ]] || { echo "no such dir: $ART" >&2; exit 1; }

echo "==> 1. eBPF source replacement"
cp "$ART/bpf/dwell_monitor.bpf.c" bpf/dwell_monitor.bpf.c

echo "==> 2. Loader patch"
patch -p1 < "$ART/bpf/loader.go.patch"

echo "==> 3. New test files"
mkdir -p test
cp "$ART/test/bench.py" test/bench.py
cp "$ART/test/test_fd_tracking.py" test/test_fd_tracking.py
chmod +x test/bench.py test/test_fd_tracking.py

echo "==> 4. STATUS.md"
cp "$ART/docs/STATUS.md" STATUS.md

echo "==> 5. README patch"
patch -p1 < "$ART/docs/README.md.patch"

echo "==> 6. CHANGELOG: prepend v1.5.0 entry under '## [Unreleased]' line"
python3 - <<'PY'
from pathlib import Path
import sys
src = Path("CHANGELOG.md")
text = src.read_text()
entry = Path(sys.argv[1]).read_text()
# Insert after the '## [Unreleased]' header block, before the next '## [' line.
import re
m = re.search(r'## \[Unreleased\].*?(?=^## \[)', text, flags=re.M | re.S)
if m:
    head = text[:m.end()]
    tail = text[m.end():]
    src.write_text(head + entry + "\n" + tail)
else:
    # Fall back: prepend after first H1
    src.write_text(text.replace("# Changelog\n", "# Changelog\n\n" + entry + "\n", 1))
print("CHANGELOG.md updated")
PY
"$ART/docs/CHANGELOG_v1.5.0_entry.md"

echo "==> 7. Documentation deletions"
rm -rf docs/archived/sessions
rm -f docs/architecture_diagram.txt

echo "==> 8. Quick sanity"
git status --short
echo
echo "Now: review with 'git diff', then:"
echo "  make clean all"
echo "  python3 test/test_fd_tracking.py   # after starting daemon"
echo "  python3 test/bench.py              # after starting daemon w/ enforcement"
echo "  git add -A && git commit -m 'v1.5.0: fix FD tracking, add benchmark, doc truth pass'"
echo "  git tag v1.5.0 && git push origin main --tags"
