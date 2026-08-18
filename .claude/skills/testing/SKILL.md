---
name: testing
description: Use when a completed stage needs verification or the finished branch needs the final test gate — the testing stages of the orchestrated workflow.
---

# Testing (project)

Nothing passes on your word — only on evidence. A test you didn't run is a test that didn't pass; never infer success from reading code.

Two modes; the dispatch names exactly one:

- **Stage mode** — scope = the stage's tasks and their file sets. Run `go build ./...`, `go vet ./...`, `go test ./...` (target packages once the module grows past one). Gap-check the diff: new branching logic, parsing, or a trust-boundary path without a test → you write the test (matching existing patterns) and run it.
- **Final mode** — the full gate over the whole branch: `gofmt -l .` (must print nothing), `go build ./...`, `go vet ./...`, `go test -race ./...`, and the smoke run — `go run . < /dev/null` must print the startup line and exit 0 (the EOF path needs no API key).

## Rules

- Evidence before claims: verbatim command output and exit codes first, then the verdict.
- Failures: reproduce at the smallest scope (`go test -run <Name> ./...`), capture the exact output, and report — fixes route through the orchestrator to the coder. Fix a test yourself only when the test itself is wrong (stale fixture, wrong expectation), and say so.
- Never weaken an assertion or delete a test to get to green — that is a Must-fix finding to report, not a fix.
- Never mock away the behavior under test.
- Never commit or stage. Never edit source outside adding missing tests.

## Report (final message, nothing after it)

```
## Test report
- Mode: stage <n> | final
- Commands run: <each with exit code>
- Results: <verbatim pass/fail summary lines>
- Tests added: <paths + what they cover, or "none">
- Failures: <exact output + suspected cause, or "none">
- Coverage gaps left: <what remains untested and why that's acceptable, or "none">

## Learnings
- <flaky behavior, env quirks, gotchas worth recording, or "none">
```
