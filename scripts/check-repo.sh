#!/bin/sh
# The repository's own invariants, run by CI and by hand. Exit 1 on the first failure.
set -eu
cd "$(dirname "$0")/.."
fail() { echo "FAIL: $1" >&2; exit 1; }
V="$(tr -d ' \n' < VERSION)"
echo "$V" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || fail "VERSION must be x.y.z, got '$V'"
grep -q "^## \[Unreleased\]" CHANGELOG.md || fail "CHANGELOG.md needs an [Unreleased] section"
grep -q "^## \[$V\]" CHANGELOG.md || fail "CHANGELOG.md has no section for $V"
for f in README.md CLAUDE.md AGENTS.md CONTRIBUTING.md LICENSE BACKLOG.md ROADMAP.md CHANGELOG.md \
  scripts/make-clips.sh testdata/clip-vp8.ivf testdata/clip-h264.h264; do
  [ -f "$f" ] || fail "$f is missing"
done
# The honest limits the README must state, in so many words.
grep -q "not glass-to-glass" README.md || fail "README.md must say latency is not glass-to-glass"
grep -q "latency: unavailable" README.md || fail "README.md must say a stripped extension gives 'latency: unavailable'"
grep -q "No-verdict rule" README.md || fail "README.md must state the no-verdict rule"
grep -q "Hosts only" README.md || fail "README.md must state that reports record hosts only"
# The threshold the README states is the one the code applies.
grep -q 'NoVerdictThreshold = 0.10' internal/report/report.go || fail "the no-verdict threshold is no longer 10% — update README.md with it"
grep -q "more than 10% of the viewers" README.md || fail "README.md must state the 10% threshold"
# The release stamps the version variable that exists.
grep -q 'internal/version.Version=' .github/workflows/release.yml || fail "release.yml must set internal/version.Version"
grep -q '^var Version = "dev"' internal/version/version.go || fail "internal/version.Version must default to dev"
# Every evals file is dated.
for f in evals/*.md; do
  [ -e "$f" ] || continue
  case "$(basename "$f")" in [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]-*.md|README.md) ;; *) fail "$f: evals files are named YYYY-MM-DD-<what>.md" ;; esac
done
echo "ok — repo invariants hold at $V"
