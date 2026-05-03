# Move dwell-fiber to chokmah-me + ship v1.5.0

This is the full sequence. Copy-paste in chunks; verify each step before
moving on. Total wall time: ~20 minutes if nothing surprising.

Assumes:
  - Local working copy of dyb5784/dwell-fiber on a clean main
  - chokmah-me/dwell-fiber created on github, EMPTY (no README/license)
  - You have push access to chokmah-me org via SSH or token
  - You have the v1.5.0/ artifact directory and the move/ artifact directory
    from this conversation accessible locally


## Step 1: Move the repo (history preserved)

```bash
cd /path/to/dwell-fiber

# Confirm starting state
git status              # should be clean
git remote -v           # origin = dyb5784/dwell-fiber

# Add new remote
git remote add chokmah git@github.com:chokmah-me/dwell-fiber.git

# Push everything: branches + tags
git push chokmah --all
git push chokmah --tags

# Visual check on github.com/chokmah-me/dwell-fiber:
#   - main exists with all your commits
#   - feature/v3-wip-architecture branch is there
#   - tags v1.0.0..v1.4.2 are there

# Switch local default
git remote remove origin
git remote rename chokmah origin
git remote -v           # confirm origin = chokmah-me/dwell-fiber

# Pull to verify auth
git fetch origin
```


## Step 2: Migrate URL references in the repo

```bash
# Make a branch for the URL migration (separate commit from v1.5.0)
git checkout -b chore/url-migration

# Run the migration script
bash /path/to/move/migrate_urls.sh

# Review what changed
git diff --stat
git diff CITATION.cff CITATION.bibtex README.md

# Commit
git add -A
git commit -m "chore: update repo URLs to chokmah-me/dwell-fiber

Migrated all references in markdown, citation files, CI configs,
shell scripts. Go module path (go.mod and Go imports) intentionally
left as github.com/dyb5784/dwell-fiber — changing it breaks every
internal import for zero functional gain. Will revisit if and when
there is external pull (paper, deployment) for the new module path."

git push -u origin chore/url-migration
```

Open the PR on github, merge to main. Or if you're solo and impatient:

```bash
git checkout main
git merge chore/url-migration --ff-only
git push origin main
git branch -d chore/url-migration
git push origin --delete chore/url-migration
```


## Step 3: Apply v1.5.0

```bash
git checkout main
git pull
git checkout -b v1.5.0

# Run the v1.5.0 apply script (separate from the move artifacts)
bash /path/to/v1.5.0/apply_v1.5.0.sh /path/to/v1.5.0

# Review
git diff --stat
git diff bpf/dwell_monitor.bpf.c   # the real correctness fix
git diff README.md                 # truth pass

# Build
make clean all
make verify

# Verify the FD-tracking fix actually works.
# Terminal 1:
sudo ./bin/dwell-fiber-daemon --enable-enforcement &
DAEMON_PID=$!

# Terminal 2 (or wait then run):
sleep 3
python3 test/test_fd_tracking.py
# Expected: 3 'High dwell:' lines in daemon output, ~3.5s, ~5.0s, ~7.0s
# Pre-fix would have shown 1 line.

# Run the benchmark
python3 test/bench.py
# Produces BENCHMARKS.md

# Stop the daemon
sudo kill $DAEMON_PID

# Commit the benchmark results too
git add BENCHMARKS.md
git commit --amend --no-edit  # or as a separate commit, your call

git push -u origin v1.5.0
```

Open PR, merge to main, then tag:

```bash
git checkout main
git pull
git tag -a v1.5.0 -m "v1.5.0: V2.x but actually correct

- Fix FD tracking bug: concurrent file opens in one process now
  produce distinct dwell events.
- Add benchmark harness with documented benign vs sustained-dwell-attack
  scenarios.
- Documentation truth pass: STATUS.md, README rewrite, CHANGELOG.
- V3.0 and Coq sub-60% explicitly marked as research-in-progress."
git push origin v1.5.0
```


## Step 4: Park the old repo

On github.com/dyb5784/dwell-fiber:
  - Settings → scroll to bottom → "Archive this repository"
  - OR: edit README to add a one-line banner at the top:

```markdown
> **This repo has moved.** See [chokmah-me/dwell-fiber](https://github.com/chokmah-me/dwell-fiber)
> for active development. This copy is preserved for incoming citation links.
```

Don't delete it. Citations break. Stack Overflow links break. Anyone
who starred or forked loses context.


## Step 5: Github release page (optional but recommended)

On chokmah-me/dwell-fiber → Releases → "Draft a new release":
  - Tag: v1.5.0 (already pushed)
  - Title: "v1.5.0: V2.x but actually correct"
  - Body: paste the v1.5.0 entry from CHANGELOG.md

This is what people see when they land on the repo. The release notes
ARE the project's public face for the next month. Make them honest.


## Sanity checks

After all of the above:

```bash
# Verify go.mod still works
cd daemon && go build . && cd ..

# Verify URLs:
grep -r 'dyb5784' --include='*.md' --include='*.cff' --include='*.bibtex' .
# Should return nothing (or only intentional references in archived/historical docs)

grep -r 'chokmah-me' --include='*.md' . | head
# Should show the new URLs in active docs
```


## What you'll have when this is done

  github.com/dyb5784/dwell-fiber          archived, points at new repo
  github.com/chokmah-me/dwell-fiber       active, at v1.5.0
    - main: v1.5.0 with FD-tracking fix, benchmark, honest docs
    - tags: full history v1.0.0..v1.5.0
    - feature/v3-wip-architecture: untouched, frozen
  CITATION.cff and .bibtex point at new URL
  go.mod still says github.com/dyb5784/dwell-fiber (intentional)

Total commits added: 2 (URL migration + v1.5.0)
Total files changed in v1.5.0 itself: ~10
Total files changed in URL migration: ~15-20 markdown/yml/etc
