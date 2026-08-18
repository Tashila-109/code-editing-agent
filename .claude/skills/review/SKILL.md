---
name: review
description: Use when a plan document, task diff, or branch diff needs independent review — the review stages of the orchestrated workflow, or any second-opinion/pre-merge review here.
---

# Review (project)

You review; you never fix. You must be a fresh agent that did not produce the artifact. Read-only: no edits, no staging, no commits, no comments posted anywhere.

## Scope modes (the dispatch names exactly one)

- **Plan** — `docs/planning/<slug>/plan.md` against its design: every design `AC-n`/`INV-n` maps to exactly one task; stages ordered by real dependency; every `Parallel: yes` stage has provably disjoint file sets and no interface dependency between its tasks; each task readable cold (a fresh agent could implement it from the task alone) and its Checks runnable as written.
- **Task** — `git diff HEAD -- <file set>` (a sibling task may be mid-edit outside those paths — never widen the scope): the changed code, judged in full-file context. Read the whole files, not just hunks; bugs live in the unchanged surroundings.
- **Branch** — `git diff $(git merge-base HEAD origin/main)...HEAD` plus anything uncommitted: cross-task consistency (naming, error-handling drift between commits), leftover debug/TODO/dead code, migrations-vs-code mismatches, and feature-level test gaps the per-task reviews couldn't see. End with:

```
## Work summary
- Shipped: <what, in plain words>
- Commits: <sha — one-line purpose, each>
- Review verdicts: <per task/stage>
- Test evidence: <what was run, result>
- Open risks: <or "none">
```

## What always gets checked (code scopes)

- Behavior against the task's cited `AC-n`/`INV-n`; affected failure paths.
- Security: this program executes model-chosen tool calls — treat tool inputs (paths, commands, patterns) as untrusted; flag any new capability without validation at that boundary.
- Regressions in surrounding code; callers of any changed shared function.
- Tests in the same diff, meaningful, and they would fail under a broken implementation.
- Matches existing `main.go` idiom; no new dependency where the stdlib does it.

## Finding format (every finding, priority order)

```
Priority: Must fix | Should fix | Could fix
Confidence: 3–5   (below 3: investigate further or mark unverified — do not report)
What: <the technical problem>
Why it matters: <impact on the user, the model loop, or the repo>
Where: <file:line, or plan section>
Suggested fix: <short, practical direction>
```

Must fix = unsafe to merge (security, data loss, broken required behavior). Should fix = real defect or reliability risk to correct before merge. Could fix = proven, limited problem or simplification — never a style preference.

## Verdict

`Approve` / `Request changes` / `Blocked`, then what remains unverified. Any Must or Should fix ⇒ `Request changes`. Always include a short **Looks good** section — required even when there are blockers. No findings at any level? Say so explicitly.

End with:

```
## Learnings
- <recurring mistake patterns worth a never-again entry, or "none">
```
