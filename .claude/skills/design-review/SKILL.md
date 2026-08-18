---
name: design-review
description: Use when a proposed design document needs independent review before planning starts — the design-review stage of the orchestrated workflow.
---

# Design review (project)

Review a proposed design before any planning or implementation. You must be a fresh agent that did not write it. Read-only: no edits to the design, the code, or anything else.

## Workflow

1. Read the design (path from the dispatch), the brief it answers, `CLAUDE.md`, `.claude/never-again.md`, and the current code. Treat the design's claims about the current system as unverified until the code confirms them.
2. Trace one real case end to end — input arrives → tool call → effect on files/system → what the user sees — including the failure and recovery legs.
3. Challenge the chosen design: is there a simpler choice reaching the same AC-n set with less state, coupling, or fewer new dependencies? Over-engineering is the house failure mode in this repo — a one-file Go CLI rarely needs new abstractions. Flag it.
4. Check the focus list. Report material open questions only when two capable implementations could answer them differently in a way that affects users, data, interfaces, security, or proof — **Blocking** (planning must not start) or **Important** (record the answer; recommend a safe default).
5. Return findings and the verdict. Do not rewrite the design or plan the work.

## Focus

- Fit with the current code: ownership, boundaries, interfaces, data, compatibility.
- Failure paths: partial failure, retry, recovery, and the tool-error path back to the model.
- Security: model-chosen tool inputs (paths, commands, patterns) are untrusted — the design must name the trust boundary and what enforces it, not assume good behavior.
- Limits and cost only where they can change the choice.
- Every INV-n and AC-n observable and provable. The implementation must never have to invent user-visible behavior, interfaces, or failure behavior.

## Findings and verdict

Use the finding format and verdict set from `.claude/skills/review/SKILL.md` (Must fix / Should fix / Could fix, confidence 3–5, Approve / Request changes / Blocked). Cite design section numbers instead of file:line where the flaw lives in the document. Always include a **Looks good** section naming what is correctly settled.

Do not approve because every template section exists — approve because the choices are safe, simple enough, and provable. State what remains unverified.

End with:

```
## Learnings
- <recurring design-defect patterns worth a never-again entry, or "none">
```
