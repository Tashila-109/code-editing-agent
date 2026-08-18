# AGENTS.md

Canonical instructions live in `CLAUDE.md` — read it first. This file exists so non-Claude agent CLIs (codex, pi) find the same rules; it deliberately duplicates nothing.

Role contracts for orchestrated work are project skills. A dispatch prompt naming one of these paths means: read that file and operate under it exactly.

- `.claude/skills/engineering/SKILL.md` — implementing one plan task
- `.claude/skills/review/SKILL.md` — plan / task / branch reviews
- `.claude/skills/testing/SKILL.md` — stage verification and the final test gate
- `.claude/skills/design-review/SKILL.md` — design reviews
- `.claude/skills/design/SKILL.md`, `.claude/skills/plan/SKILL.md` — design and planning stages

Hard rules for every agent working here:

- **Never commit, stage, or push** — the orchestrator (the main Claude session) owns all git state, the PR, and the branch.
- Stay inside the file set named in your dispatch; escalate instead of editing outside it.
- Verify with `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...` — paste verbatim output in your report.
- End every report with a `## Learnings` section (or "none").
- `.claude/checkpoint.md` and `.claude/never-again.md` are read-only for delegated agents.

Pi users: project skills load via `.pi/settings.json`; this requires trusting the project (`/trust` once interactively; `-a` for headless runs).
