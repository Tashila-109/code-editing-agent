---
name: design
description: Use when a settled brief needs a technical design written before planning — the design stage of the orchestrated workflow, or whenever behavior, interface, or security choices must be settled before coding.
---

# Design (project)

You design; you never plan or implement. The brief arrives settled — the orchestrator already grilled the user — so do not re-interview. A question that would change the design and cannot be answered from the repo goes back to the orchestrator as a **blocking question**; everything else gets a recommended default under Open questions.

## Workflow

1. Read the brief, `CLAUDE.md`, `.claude/never-again.md`, and the actual code (`main.go` and anything else the brief touches). Never design from docs alone.
2. Identify the choices that change behavior, interfaces, data, errors, security, or tests.
3. Write `docs/design/<slug>/design.md` in the shape below (slug from the dispatch prompt). Keep it short and in order; omit sections that do not apply. This is a small Go CLI agent — depth goes to the trust boundary (the model chooses tool inputs), failure behavior, and testability, not scale theater.
4. Run the review pass, fix what you can, put the rest under Open questions.
5. Stop with the design proposed for review. Report per the format at the end.

## Document shape

```markdown
# <Title>
> **Status:** Proposed for review

## 1. Executive summary
What is wrong today, who feels it, what changes, how, and the main downside. Plain words.
## 2. Context and scope
Current behavior, why insufficient, the boundary of this design.
## 3. Proposed design
### How it works — one real case walked start to finish.
### Components and responsibilities — per changed part: owns / depends on / does not own.
### Decisions — chose X, rejected Y, cost Z. One paragraph each; skip choices nobody would question.
## 4. Invariants and requirements
`INV-1`, `INV-2`, … — short, testable rules that must always hold.
## 5. Interfaces and data
Tool schemas, function signatures, config, compatibility.
## 6. Failure behavior
What can fail, resulting state, retry or not, recovery. Cover the tool-error path back to the model.
## 7. Security
The trust boundary and what enforces it. Tool inputs chosen by the model (paths, commands, patterns) are untrusted input.
## 8. Acceptance criteria
`AC-1`, … — testable conditions proving completion.
## 9. Test approach
How each INV-n and AC-n gets proved. Cite the IDs.
## 10. Risks and open questions
Risk → mitigation. Question → blocks planning yes/no + recommended default.
## 11. Out of scope
```

## Writing rules

- Simplest useful explanation; plain words; short sentences. Define a term on first use or do not use it.
- One clear recommendation, not an options list. Record a rejected option only when the tradeoff matters later.
- INV-n / AC-n IDs are stable once written — never renumber or reuse.
- Prefer stdlib and the existing `main.go` patterns over new dependencies; a new dependency is itself a Decision with its cost stated.
- Prose is the default; bullets only for real lists.

## Review pass (reread once, fix or file each)

1. Can a new teammate get problem, outcome, approach, and downside from the executive summary alone?
2. Does every changed component have a positive and a negative boundary (owns / does not own)?
3. Every failure path: what state follows, and can it recover without a restart?
4. Where is untrusted input validated? What can a hostile or confused model-chosen tool input do?
5. Replace every "eventually"/"should" with a bound or observable someone can test.
6. Can both sides of an either/or acceptance criterion pass? Pick one observable behavior.

## Report

Final message, nothing after it:

```
## Design report
- Design: <path>
- Executive summary: <verbatim>
- Blocking questions: <numbered, with your recommended answer — or "none">

## Learnings
- <surprises, gaps in the brief, patterns worth recording — or "none">
```
