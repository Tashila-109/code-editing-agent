---
name: orchestrate
description: Use when a feature request, bugfix, or any multi-file task arrives, or the user asks to run the agentic workflow. Not for trivial single-file fixes — say so and just do those.
---

# Orchestrate

You (the main session) are the **orchestrator**: you hold primary context, route work to delegated agents, integrate results, and decide. You are the **only writer** of git state (branch, commits, push, PR) and the memory files. You never design, plan, code, review, or test in your own context. Orchestration always runs here, never inside a subagent — the approval gates and Bugbot routing need direct user interaction.

## Routing matrix (authoritative)

| Stage | Delegate | Model | Contract |
| --- | --- | --- | --- |
| Intake grilling | you + user (main session) | — | user-scope `grilling` skill |
| Design | `claude` CLI pane via Herdr | `claude-fable-5` | `.claude/skills/design/SKILL.md` |
| Design review | `codex` CLI pane | codex default | `.claude/skills/design-review/SKILL.md` |
| Plan + visual plan | `claude` CLI pane | session default | `.claude/skills/plan/SKILL.md` |
| Plan review | `codex` CLI pane | codex default | `.claude/skills/review/SKILL.md` (plan scope) |
| Coding (per task) | `pi` CLI pane(s), max 2 live (`pi-1`, `pi-2`) | pi default | `.claude/skills/engineering/SKILL.md` |
| Task review | `codex` CLI pane | codex default | `.claude/skills/review/SKILL.md` (task scope) |
| Stage testing | `pi` CLI pane (`pi-test`) | pi default | `.claude/skills/testing/SKILL.md` (stage mode) |
| Final branch review + work summary | `codex` | codex default | `.claude/skills/review/SKILL.md` (branch scope) |
| Final test gate | Claude subagent (in-process, Agent tool) | `opus` | `.claude/skills/testing/SKILL.md` (final mode) |
| Bugbot review | GitHub PR (Bugbot app) | — | findings routed by you to the `pi` fix loop |
| Branch, commits, push, PR, memory | you only | — | this file |

## Retry → fallback (every delegated dispatch)

1. A dispatch has failed when `herdr agent start` errors, the prompt returns `agent_prompt_stalled`, the agent errors out, or it ends `blocked` with nothing you can answer. First failure → retry once: fresh pane, same contract, prompt prefixed with one line naming what failed.
2. Second failure → in-process Claude subagent via the Agent tool: prompt = "Read `<contract path>` and operate under it exactly." plus the same task specifics; model per the matrix (design fallback: `fable`; others inherit the session). Note every fallback in your status text and in the close-out checkpoint entry.
3. Not inside Herdr at all (`test "${HERDR_ENV:-}" = 1` fails) → run the whole matrix as Claude subagents. **No delegated CLI is ever load-bearing.**

## Lifecycle

1. **Intake** — restate the ask. Trivial single-file fix → no orchestration; say so and do it. Otherwise run the `grilling` skill with the user in rounds; the settled decision tree becomes the design brief. Settle approval-gate preferences in the grill: the plan gate (step 10) is on by default; the design gate (step 6) is OFF unless the user explicitly asks for one.
2. **Memory bootstrap** — read `.claude/never-again.md` and the recent `.claude/checkpoint.md` entries; pass the relevant excerpts into every dispatch prompt.
3. **Branch** — require a clean tree; on `main`, `git pull`, then `git checkout -b <type>/<slug>` (`feat|fix|chore`, short kebab slug). Nothing downstream runs on `main`. `<slug>` names the doc dirs below.
4. **Design** — dispatch the designer (`claude`, Fable 5): project `design` skill with the brief → `docs/design/<slug>/design.md`. Blocking questions come back to you; put them to the user, redispatch with the answers.
5. **Design review** — dispatch codex on the design-review contract, naming the design path and the code it makes claims about. Must/Should-fix findings → designer revision → re-review. Never skip re-review after fixes.
6. **Design gate (opt-in)** — only when the user asked for it during grilling: present the design and STOP for an explicit go. Otherwise relay the approved design's executive summary in your status text and proceed straight to planning.
7. **Plan** — dispatch the planner (`claude`): project `plan` skill, design path in the prompt → `docs/planning/<slug>/plan.md` with stages, tasks, and per-task file sets.
8. **Plan review** — codex, review contract, plan scope. Findings → planner → re-review.
9. **Visual plan** — same planner pane: user-scope `visual-plan` skill over the approved plan; relay the local bridge URL to the user.
10. **Approval gate 2** — present the plan + visual plan URL and STOP for an explicit go.
11. **Implement, stage by stage** — for each stage in order: dispatch one `pi` coder per task (max 2 live), each prompt scoped to exactly its task and file set. A stage marked `Parallel: no` runs its tasks sequentially. Pipeline, don't batch: the moment a task lands, dispatch its codex review (task scope = `git diff HEAD -- <task file set>`) while the sibling keeps coding. Findings → same coder → re-review.
12. **Stage testing** — when every task in the stage is reviewed clean and no coder is mid-edit (quiet tree), dispatch `pi-test` on the testing contract, stage mode. Failures → coder (verbatim output in the prompt) → re-review → re-test.
13. **Commit** — after zero Must/Should-fix and a green stage test: you commit each task's file set separately, serialized, ending the message with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`. Delegates never commit, stage, or push. New implementer edits after a review → re-review the delta before committing; tests the tester added under its own contract ride on its green report and need no re-review.
14. **Final branch review** — after the last stage's commits: codex, review contract, branch scope (`git diff $(git merge-base HEAD origin/main)...HEAD` plus anything uncommitted), including the `## Work summary` section. Findings → fix loop (steps 11–13).
15. **Final test gate** — Claude subagent, model `opus`, testing contract in final mode. Always runs; never skipped on multi-stage work.
16. **Push + PR** — `git push -u origin <branch>`; `gh pr create` — title from the design's executive summary, body = the work summary + test evidence, ending with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
17. **Bugbot** — if it didn't trigger on PR creation, comment `bugbot run` (`gh pr comment <num> --body "bugbot run"`). Poll for Bugbot's review (`gh pr view <num> --comments`, `gh api repos/{owner}/{repo}/pulls/<num>/comments`) at a sane interval. Each finding → `pi` coder fix (scoped to the finding's files) → codex re-review → `pi-test` when more than trivial → commit + push, which hands Bugbot the new commits. Loop until clean or the user dismisses the remainder. No Bugbot activity after ~10 minutes → note it and fall back to one fresh Claude subagent adversarial review of the PR diff.
18. **Close-out** — prepend a `checkpoint.md` entry; append `never-again.md` entries for real mistakes surfaced in any report's `## Learnings`; hand the user the PR URL, commit list, and test evidence.

## Parallelism rules

- Same checkout, no worktrees: parallel tasks are safe only because the plan marks their file sets disjoint — you enforce it. Overlapping file sets or an interface dependency = sequential, no exceptions.
- Task reviews are path-scoped, so they may run while a sibling codes. Tests and commits need a quiet tree.
- Unique live-agent names throughout: `designer`, `planner`, `pi-1`, `pi-2`, `pi-test`, `codex-review-1`, …

## Herdr mechanics

- Two-column layout: column 1 = this session; column 2 = delegated CLI panes stacked downward. First pane: `herdr pane split --current --direction right --cwd "$PWD" --no-focus`; every later pane splits the newest column-2 pane `--direction down`. Pane id from `.result.pane.pane_id` — parse JSON, never guess ids. Rename each pane to its role.
- Pane hygiene — close each delegate's pane the moment its engagement is fully done, before the next stage starts: designer + design-review panes once the design is approved; the planner pane right after the plan gate (its visual-plan serve process dies with it, so never close it before the user has seen the URL); coder panes after their task commits; review/test panes once their verdict is consumed. An errored or `blocked` pane stays open for inspection — tell the user.
- Start + prompt:

```bash
herdr agent start pi-1 --kind pi --pane <pane_id>
herdr agent start designer --kind claude --pane <pane_id> -- --model claude-fable-5
herdr agent prompt pi-1 "<task + contract path + scope + report format>" --wait --timeout 600000
herdr agent read pi-1 --source visible --lines 200
```

- CLI panes have no session context: every prompt is self-contained — contract path, branch, doc paths, task/stage, memory excerpts, required report format.
- TUI agents run on the alternate screen: read with `--source visible`; if the response outruns the viewport, ask the agent to write it to a temp markdown file and reply with the path.
- `blocked` or `agent_prompt_stalled` → `herdr agent get` + `herdr agent read` before sending anything.
- One-time Pi setup: the project must be trusted (`/trust` interactively once; `-a` for headless).

## Memory (close-out only, you only)

- `.claude/checkpoint.md`: prepend one entry per the template in its header; keep the last 5, move older ones to `.claude/memory/archive/`.
- `.claude/never-again.md`: append-only — date, incident, root cause, prevention. Real mistakes only, not routine friction.
