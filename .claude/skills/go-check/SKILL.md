---
name: go-check
description: Run go build, vet, and tests with race detector and coverage for this project. Use this before committing, after refactoring, or any time you want a full health check.
user-invocable: true
allowed-tools: Bash
---

Run the following checks in order, stopping and reporting if any step fails:

1. **Build**: `go build ./...`
2. **Vet**: `go vet ./...`
3. **Test with race detector and coverage**: `go test -race ./... -cover`

Report results clearly:
- If all pass, summarise the coverage figures per package.
- If any step fails, show the full error output and identify which package(s) are broken.
- Do not attempt to fix anything — just report.
