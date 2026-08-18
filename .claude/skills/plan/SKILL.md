---
name: plan
description: Use when an approved design needs breaking into staged tasks for separate agent runs — the planning stage of the orchestrated workflow. Not for one coding task or its short execution outline.
---

# Plan (project)

Turn an approved design into ordered, staged tasks that separate agents implement cold. You plan; you never implement. If an unresolved choice would change behavior, interfaces, data, security, or proof, stop and return it to the orchestrator — that is a design gap, not a planning call.

## Workflow

1. Read the design (path from the dispatch), `CLAUDE.md`, and the code the design touches.
2. Break the work into tasks. Each task: fits one agent run, delivers working behavior (tests in the same task), sized for exactly one commit. Split refactoring out only when it would hide the behavior change.
3. Group tasks into ordered **stages** and mark parallelism per the rules below.
4. Write `docs/planning/<slug>/plan.md`: a header block (design path, stage table: stage → tasks → parallel yes/no), then the tasks.
5. Stop and report. After the plan passes review, the orchestrator redispatches you once more: run the user-scope `visual-plan` skill over the approved plan (lanes = stages, parallel tasks side by side) and report the local bridge URL.

## Stage rules

- Stages are ordered by real dependency; a stage starts only when the previous one is committed.
- `Parallel: yes` only when every pair of tasks in the stage has **disjoint file sets** and neither task consumes an interface, type, or function the other defines. When in doubt: `Parallel: no`.
- At most 2 tasks run live at once — never contort the breakdown to manufacture a third lane.

## Task template (every task — a cold handoff; assume the implementer has seen nothing)

```markdown
## <Task title — action and result in plain words>
Stage: <n> · Parallel-safe: yes|no

### Summary
Current behavior, who it affects, what changes, why it matters. 2–4 sentences.

### File set (REQUIRED)
Explicit paths created or edited. Reviews and parallel dispatch enforce on this list.

### Depends on
Other task titles, or `None`.

### Context
What a fresh agent needs with no session history: the relevant parts of the system, prior tasks' outputs, source decisions. Link the design for detail, not orientation.

### Constraints
Decided choices that must not change.

### Acceptance criteria
Testable conditions. Cite every applicable `AC-n` / `INV-n` without changing their meaning.

### Checks
Exact commands (`go build ./...`, `go vet ./...`, `go test ./...`, plus task-specific runs).

### Out of scope
Related work this task must not absorb.
```

## Boundaries

- No file-split or layer-split tasks that don't each deliver working behavior.
- No scaffolding or cleanup tasks without a checked outcome.
- Preserve design IDs so review and testing can trace proof back to the design.
- Plain words in titles and summaries; define project terms on first use.
- Reread each task alone before reporting: a cold reader must be able to say why it exists, what to change, and how to prove it works.

## Report

```
## Plan report
- Plan: <path>
- Stages: <stage → task titles → parallel yes/no, one line each>
- Visual plan: <bridge URL when dispatched for it — otherwise "pending review">

## Learnings
- <or "none">
```
