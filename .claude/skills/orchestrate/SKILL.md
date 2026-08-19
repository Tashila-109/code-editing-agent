---
name: orchestrate
description: Use when a feature request, bugfix, or any multi-file task arrives, or the user asks to run the agentic workflow. Not for trivial single-file fixes — say so and just do those.
---

# Orchestrate

You (the main session) are the **orchestrator**: you hold primary context, route work to delegated agents, integrate results, and decide. You run on **Fable 5 at high effort** (this project's main model). You are the **only writer** of git state (branch, commits, push, PR) and the memory files. You never design, plan, code, review, or test in your own context. Orchestration always runs here, never inside a subagent — the approval gates and human review need direct user interaction, per Reporting mode below.

## Reporting mode

- **Direct interactive use** (default): a human runs `claude` in this repo. Every approval gate and the final report go to that human in this session, exactly as written below.
- **Launched by a Master Chief/Hermes parent**: your initial prompt names a FLEETCOM task, a Chief/Hermes parent, and a report contract. You are still this same orchestrator running this same routing matrix and lifecycle — you still own this repo's branch/git state per CLAUDE.md. Every approval gate presents its decision package here and pauses for that parent's response instead of a human's; the close-out report (step 20) goes to that parent. Never invent the parent's answer at a gate — pause and wait for it, same as you would a human.

## Model rule

- **Orchestrator (this session)**: Fable 5, high effort.
- **Final gate — tester and adversarial review**: `opus` (the only Opus agents; the final gate is the workflow's highest-value check).
- **Designer and planner**: `opus` (Opus 5).
- **Every other Claude agent you spawn** (any Claude-subagent fallback): `sonnet`. Pi and codex agents bring their own models — these rules apply only to agents whose model you set.

## Routing matrix (authoritative)

| Stage | Delegate | Model | Contract |
| --- | --- | --- | --- |
| Intake grilling | you + user (main session) | Fable 5 / high | user-scope `grilling` skill, questions via AskUserQuestion |
| Design | `claude` CLI pane via Herdr | `opus` | `.claude/skills/design/SKILL.md` |
| Design review | `codex` CLI pane | codex default | `.claude/skills/design-review/SKILL.md` |
| Plan + visual plan | `claude` CLI pane | `opus` | `.claude/skills/plan/SKILL.md` |
| Plan review | `codex` CLI pane | codex default | `.claude/skills/review/SKILL.md` (plan scope) |
| Coding (per task) | `pi` CLI pane(s), max 2 live (`pi-1`, `pi-2`) | pi default | `.claude/skills/engineering/SKILL.md` |
| Task review | `codex` CLI pane | codex default | `.claude/skills/review/SKILL.md` (task scope) |
| Stage testing | `pi` CLI pane (`pi-test`) | pi default | `.claude/skills/testing/SKILL.md` (stage mode) |
| Final branch review + work summary | `codex` | codex default | `.claude/skills/review/SKILL.md` (branch scope) |
| Final test gate | Claude subagent (in-process, Agent tool) | `opus` | `.claude/skills/testing/SKILL.md` (final mode) |
| Final gate review (adversarial) | Claude subagent (in-process, Agent tool) | `opus` | adversarial break-it review of the whole PR diff |
| Hunk review (final human gate) | Herdr tab you launch, focused | — | user-scope / bundled `hunk-review` skill |
| Bugbot review | GitHub PR (Bugbot app) | — | **opt-in only** — run when the user explicitly asks |
| Branch, commits, push, PR, memory | you only | Fable 5 / high | this file |

## Retry → fallback (every delegated dispatch)

1. A dispatch has failed when `herdr agent start` errors, the prompt returns `agent_prompt_stalled`, the agent errors out, or it ends `blocked` with nothing you can answer. First failure → retry once: fresh pane, same contract, prompt prefixed with one line naming what failed.
2. Second failure → in-process Claude subagent via the Agent tool: prompt = "Read `<contract path>` and operate under it exactly." plus the same task specifics; model `sonnet` (designer, planner, final gate tester, and final gate review are the Opus exceptions). Note every fallback in your status text and in the close-out checkpoint entry.
3. Not inside Herdr at all (`test "${HERDR_ENV:-}" = 1` fails) → run the whole matrix as Claude subagents (Sonnet, bar the Opus final gate). **No delegated CLI is ever load-bearing.**

## Lifecycle

1. **Intake** — restate the ask. Trivial single-file fix → no orchestration; say so and do it. Otherwise run the `grilling` skill with the user in rounds, asking every grill question through the **AskUserQuestion tool** — concrete, mutually exclusive options per question (recommended option first, marked "(Recommended)"; `multiSelect` where choices compose) so the user answers by keyboard selection, never by typing; the built-in "Other" covers free-text answers. Only a question that genuinely has no enumerable options may be asked as plain text. The settled decision tree becomes the design brief. Settle approval-gate preferences in the grill: the plan gate (step 10) is on by default; the design gate (step 6) is OFF unless the user explicitly asks for one.
2. **Memory bootstrap** — read `.claude/never-again.md` and the recent `.claude/checkpoint.md` entries; pass the relevant excerpts into every dispatch prompt.
3. **Branch** — require a clean tree; on `main`, `git pull`, then `git checkout -b <type>/<slug>` (`feat|fix|chore`, short kebab slug). Nothing downstream runs on `main`. `<slug>` names the doc dirs below.
   Then **arm the pane monitor** (once per run, before the first dispatch): a persistent background `Monitor` polling the delegate panes so a `blocked` (permission prompt) or errored delegate notifies you instead of being discovered on a `wait`. See Herdr mechanics.
4. **Design** — dispatch the designer (`claude`, `opus`, `--permission-mode auto`): project `design` skill with the brief → `docs/design/<slug>/design.md`. Blocking questions come back to you; put them to the user, redispatch with the answers.
5. **Design review** — dispatch codex on the design-review contract, naming the design path and the code it makes claims about. Must/Should-fix findings → designer revision → re-review. Never skip re-review after fixes.
6. **Design gate (opt-in)** — only when the user asked for it during grilling: present the design and STOP for an explicit go. Otherwise relay the approved design's executive summary in your status text and proceed straight to planning.
7. **Plan** — dispatch the planner (`claude`, `opus`, `--permission-mode auto`): project `plan` skill, design path in the prompt → `docs/planning/<slug>/plan.md` with stages, tasks, and per-task file sets.
8. **Plan review** — codex, review contract, plan scope. Findings → planner → re-review.
9. **Visual plan** — same planner pane: user-scope `visual-plan` skill over the approved plan; relay the local bridge URL to the user.
10. **Approval gate 2** — present the plan + visual plan URL and STOP for an explicit go.
11. **Implement, stage by stage** — for each stage in order: dispatch one `pi` coder per task (max 2 live), each prompt scoped to exactly its task and file set. A stage marked `Parallel: no` runs its tasks sequentially. Pipeline, don't batch: the moment a task lands, dispatch its codex review (task scope = `git diff HEAD -- <task file set>`) while the sibling keeps coding. Findings → same coder → re-review.
12. **Stage testing** — when every task in the stage is reviewed clean and no coder is mid-edit (quiet tree), dispatch `pi-test` on the testing contract, stage mode. Failures → coder (verbatim output in the prompt) → re-review → re-test.
13. **Commit** — after zero Must/Should-fix and a green stage test: you commit each task's file set separately, serialized, ending the message with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`. Delegates never commit, stage, or push. New implementer edits after a review → re-review the delta before committing; tests the tester added under its own contract ride on its green report and need no re-review.
14. **Final branch review** — after the last stage's commits: codex, review contract, branch scope (`git diff $(git merge-base HEAD origin/main)...HEAD` plus anything uncommitted), including the `## Work summary` section. Findings → fix loop (steps 11–13).
15. **Final test gate** — Claude subagent, model `opus`, testing contract in final mode. Always runs; never skipped on multi-stage work.
16. **Final gate review** — Claude subagent, model `opus`: a fresh adversarial break-it review of the whole branch diff, told to *find real bugs the pipeline missed* (concurrency/timeout races, trust-boundary exploits from model-chosen inputs, resource leaks) and to reproduce each with a runnable probe in a temp dir (no tracked-file edits, never commit). This step is standard, not a fallback — the dry run proved codex reviews and the Opus test gate can both pass while Must-fix bugs survive. Findings → fix loop (steps 11–13) → re-run this gate and the final test gate if the fix is broad.
17. **Push + PR** — `git push -u origin <branch>` (push `main` first if `origin/main` is behind, so the PR diff is only the feature); `gh pr create` — title from the design's executive summary, body = the work summary + test evidence, ending with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
18. **Hunk review (final human gate)** — the last step, once every automated gate is green and the PR is up. Launch Hunk in a **new, focused** Herdr tab showing the branch diff for the user's manual review before they accept the PR. Create the tab, run `hunk diff origin/main...HEAD` in its pane, and tell the user it is ready. Drive/annotate the live session only on request via `hunk session *` commands (contract: `hunk skill path`). Do not run `hunk diff` in your own pane — it is the user's TUI.
19. **Bugbot (opt-in only)** — do NOT run by default. Only when the user explicitly asks: comment `bugbot run` on the PR (`gh pr comment <num> --body "bugbot run"`), poll for its review, and route each finding → `pi` fix → codex re-review → `pi-test` → commit + push.
20. **Close-out** — prepend a `checkpoint.md` entry; append `never-again.md` entries for real mistakes surfaced in any report's `## Learnings`; stop the pane monitor (`TaskStop`); hand the user the PR URL, commit list, and test evidence.

## Parallelism rules

- Same checkout, no worktrees: parallel tasks are safe only because the plan marks their file sets disjoint — you enforce it. Overlapping file sets or an interface dependency = sequential, no exceptions.
- Task reviews are path-scoped, so they may run while a sibling codes. Tests and commits need a quiet tree.
- Unique live-agent names throughout: `designer`, `planner`, `pi-1`, `pi-2`, `pi-test`, `codex-review-1`, …

## Herdr mechanics

### Three-column layout

Column 1 (leftmost) = **this orchestrator session + testing panes** (`pi-test`). Column 2 = **design / planning / review panes** (`designer`, `planner`, every `codex-review-*`). Column 3 = **coding panes** (`pi-1`, `pi-2`, and fix coders). Panes stack downward within a column.

- First pane in a column splits right off the column to its left; later panes in the same column split `--direction down` off that column's newest pane. Open each column's first pane the first time that column is needed:
  - Column 2 first pane: `herdr pane split --current --direction right --cwd "$PWD" --no-focus`.
  - Column 3 first pane: split right off a column-2 pane.
  - A testing pane: split down off this orchestrator pane (stays in column 1).
- Pane id from `.result.pane.pane_id` — parse JSON, never guess ids. Track each column's newest pane id; recover it with `herdr pane layout` / `herdr pane list` rather than guessing. Rename each pane to its role.
- Pane hygiene — close each delegate's pane the moment its engagement is fully done, before the next stage starts: designer + design-review panes once the design is approved; the planner pane right after the plan gate (its visual-plan serve process dies with it, so never close it before the user has seen the URL); coder panes after their task commits; review/test panes once their verdict is consumed. An errored or `blocked` pane stays open for inspection — tell the user.

### Pane monitor (always on, armed at step 3)

Run one persistent background monitor for the whole run so delegate stalls surface immediately instead of on a blocking `wait`:

```bash
Monitor(persistent: true, description: "delegate panes (blocked/error)", command: '
  prev=""
  while true; do
    cur=$(herdr agent list 2>/dev/null | jq -r --arg ws "$HERDR_WORKSPACE_ID" \
      '.result.agents[] | select(.workspace_id==$ws and (.agent_status=="blocked" or .agent_status=="error")) | "\(.name // .pane_id): \(.agent_status)"' | sort)
    comm -13 <(printf "%s" "$prev") <(printf "%s" "$cur")
    prev=$cur; sleep 20
  done')
```

Each emitted line means a delegate needs you (usually a permission prompt) — inspect with `herdr agent get`/`read` and clear it. Stop it at close-out with `TaskStop`.

### Start + prompt

```bash
herdr agent start pi-1 --kind pi --pane <pane_id>
herdr agent start designer --kind claude --pane <pane_id> -- --model opus --permission-mode auto
herdr agent prompt pi-1 "<task + contract path + scope + report format>" --wait --timeout 600000
herdr agent read pi-1 --source visible --lines 200
```

- Spawned `claude` sessions (designer, planner) start with `--model opus --permission-mode auto`.
- CLI panes have no session context: every prompt is self-contained — contract path, branch, doc paths, task/stage, memory excerpts, required report format.
- TUI agents run on the alternate screen: read with `--source visible`; if the response outruns the viewport, ask the agent to write it to a temp markdown file and reply with the path.
- `blocked` or `agent_prompt_stalled` → `herdr agent get` + `herdr agent read` before sending anything.
- One-time Pi setup: the project must be trusted (`/trust` interactively once; `-a` for headless).

### Hunk review tab (final human gate)

Launch the review in a new **focused** tab in this workspace and run the viewer in its root pane:

```bash
herdr tab create --workspace "$HERDR_WORKSPACE_ID" --cwd "$PWD" --label hunk-review   # focused (no --no-focus); root pane id ← .result.root_pane.pane_id
herdr pane run <root_pane_id> 'hunk diff origin/main...HEAD'
```

Tell the user the tab is ready. Control the live session only on request via `hunk session *` (never run `hunk diff` in your own pane — the TUI is theirs). Outside Herdr, tell the user to run `hunk diff origin/main...HEAD` themselves.

## Memory (close-out only, you only)

- `.claude/checkpoint.md`: prepend one entry per the template in its header; keep the last 5, move older ones to `.claude/memory/archive/`.
- `.claude/never-again.md`: append-only — date, incident, root cause, prevention. Real mistakes only, not routine friction.
