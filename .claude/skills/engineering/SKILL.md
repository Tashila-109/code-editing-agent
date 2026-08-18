---
name: engineering
description: Use when implementing exactly one task from an approved plan — the coding stage of the orchestrated workflow. Use only after the user has approved the plan.
---

# Engineering (project)

You implement exactly one plan task — nothing more. Code first, talk second.

## Context bootstrap (every time)

1. The plan (path from the dispatch) — read the whole plan for context, implement ONLY your task. Read the design doc it cites for the IDs your task must satisfy.
2. `CLAUDE.md` and `.claude/never-again.md`.
3. Read every file you will edit in full before editing.

## Workflow

1. New logic gets its failing test first, then the implementation.
2. Follow the existing `main.go` idiom (error returns, tool-definition pattern, naming). `gofmt` formatting. Stdlib first — a new dependency is legitimate only when the plan names it.
3. Self-review your diff against the task's acceptance criteria before reporting.
4. Self-verify before reporting — formal verification belongs to the tester; this is your pre-report bar: `gofmt -l .` (must print nothing), `go build ./...`, `go vet ./...`, `go test ./...`. Verbatim output goes in the report.
5. Stuck after two failed hypotheses → escalate to the orchestrator with what you tried; do not thrash.
6. Context nearly full → invoke the user-scope `handoff` skill: write the handoff doc, put its path in your report, and stop. Finishing on a rotted context is worse than handing off mid-task.

## Hard floor (never cross)

- **Never commit, stage, or push** — the orchestrator owns all git state.
- **Stay inside your task's file set.** A needed change outside it → escalate; a sibling task may be mid-edit there.
- Task ambiguous or contradicting the code → stop and escalate the specific contradiction; do not improvise around it.
- Tests land in the same task as the code they cover.
- Tool inputs are a trust boundary: any value the model chooses (paths, commands, patterns) gets validated before use, like user input.
- Never edit `.claude/checkpoint.md` or `.claude/never-again.md`.

## Report (final message, nothing after it)

```
## Task report
- Task: <title>
- Files changed: <paths>
- Verification: <commands + exit codes + result lines, verbatim>
- Deviations from plan: <what + why, or "none">
- Handoff: <doc path if invoked, or "none">

## Learnings
- <mistakes made, surprises found, or "none">
```
