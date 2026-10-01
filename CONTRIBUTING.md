# Contributing

## Local loop

```bash
go vet ./... && go test -race -count=1 ./...   # unit tests + real round trips through internal/testserver
golangci-lint run ./...
./scripts/check-repo.sh                        # VERSION ↔ CHANGELOG, the statements the README must carry
node scripts/leakcheck.mjs                     # tracked files and the whole history
npm install && npm run backlog && npm run build:site   # tooling only: backlog lint, the site from README
```

No server is needed to develop. The end-to-end tests start `internal/testserver`, a
WHIP/WHEP relay on pion, behind an `httptest` server on loopback; the clients run with
loopback-only ICE, so nothing leaves the machine.

## Changing a measurement

A number in a report is a definition first and code second.

1. Write the definition where a reader meets it: the README table, the `Method` lines in
   `internal/report/report.go`, and the package comment of the code that computes it.
2. Test it on synthetic input before any network is involved — `internal/rtpstats` and
   `internal/stats` are pure arithmetic for that reason.
3. Never report a number the measurement cannot support. When an input is missing — a
   stripped header extension, clocks that disagree, too many viewers failing — the
   report says *unavailable* or *no verdict*, with the reason.
4. Nothing that could carry a token reaches a report, an error or the console: hosts
   only.

## Live runs

Only against servers you run, or a managed service on your own account within its
terms, or with written permission. A run worth keeping goes in `evals/` as a dated
Markdown file with the server's version and host — never a path, a key, or anyone's
hostname — plus the raw reports beside it.

## Backlog, roadmap, issues

`BACKLOG.md` is the single source of truth; `ROADMAP.md` is generated from it and the
GitHub issues are synced from it one way on every push to `main` that touches the file.
Items carry a stable `WB-n` id and `<!-- wb: prio= size= labels= [ver=] -->`.

## Pull requests

`main` is protected: pull request, green CI, no direct pushes or force pushes. A
conventional subject with the `WB-n` id, and a CHANGELOG line under `[Unreleased]` in
the same pull request. No tool-attribution trailers or footers.

## Releasing

The version lives in `VERSION`; the tag is `v` followed by it.

```bash
# bump VERSION; rename CHANGELOG's [Unreleased] to [x.y.z] — date, open a new [Unreleased];
# ver=main → ver=x.y.z in BACKLOG.md; regenerate the roadmap; land it by pull request
git checkout main && git pull
git tag v$(cat VERSION) && git push origin v$(cat VERSION)
```

`release.yml` verifies the tag against `VERSION`, runs the race tests, builds static
binaries for linux and darwin on amd64 and arm64 with the version baked in, attaches them
with a checksums file to the GitHub release (notes from the CHANGELOG section), attests
their build provenance, and closes the milestone whose title starts with `v<version>`.
Verify a download with `gh attestation verify whipbench-v<version>-linux-amd64 --repo
Allan-Nava/whipbench`. `release-drift.yml` fails when `main` carries a `VERSION` with no
tag for two hours, unless its CHANGELOG section opens with "Not released". Re-run a
release with `gh workflow run Release -f tag=v<version>`.
