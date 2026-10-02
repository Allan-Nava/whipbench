# AGENTS.md

The guidance for coding agents in this repository lives in [CLAUDE.md](CLAUDE.md): what
whipbench is, the six rules the code encodes (definitions first, never a fake number,
the no-verdict rule, hosts only, delay is not glass-to-glass, pure Go), the dated
facts the code depends on, how to verify a change, and the publishing hygiene. Read it
before editing.

The short version of the checks:

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./...; echo "exit $?"
golangci-lint run ./...; echo "exit $?"
./scripts/check-repo.sh; echo "exit $?"
node scripts/leakcheck.mjs; echo "exit $?"
npm run backlog && npm run build:site
```
