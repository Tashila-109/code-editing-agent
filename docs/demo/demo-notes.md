# Agentic Workflow Demo — Notes

Demo of the orchestrated multi-CLI workflow in `code-editing-agent`, showing both entry paths:
**Path 1** — direct interactive Claude session; **Path 2** — launched by the Hermes/Chief parent.

## What the audience sees

One feature request goes in; the orchestrator (Claude, Fable 5) never writes code itself. It:

1. **Grills** the requester via AskUserQuestion (keyboard-selectable options) until the brief is settled.
2. **Dispatches a designer** (Claude CLI pane, Opus 5) → `docs/design/<slug>/design.md`.
3. **Dispatches a design review** (codex pane) → must-fix findings loop back to the designer.
4. **Plans** (Opus 5) → `docs/planning/<slug>/plan.md` + visual plan URL → **plan approval gate** (stops for a human/parent "go").
5. **Implements** per task via `pi` coder panes (max 2 parallel, disjoint file sets), each task codex-reviewed and stage-tested (`pi-test`).
6. **Final gates**: codex branch review → Opus final test gate → Opus adversarial break-it review.
7. **Commits/push/PR** — orchestrator only; delegates never touch git.
8. **Hunk review tab** — focused Herdr tab with the branch diff as the final human gate.

Herdr layout: 3 columns — orchestrator+testing | design/plan/review | coders. A persistent pane
monitor surfaces blocked/errored delegates without polling manually.

## Pre-demo checklist

- [ ] Repo on `main`, clean tree, at the demo baseline (basic `main.go` only — `read_file`, `list_files`, `edit_file`; no `run_command.go` / `search_files.go`).
- [ ] Running inside Herdr (`echo $HERDR_ENV` → `1`); outside Herdr everything falls back to in-process subagents (still works, less visual).
- [ ] `ANTHROPIC_API_KEY` set; `codex` and `pi` CLIs on PATH; pi has trusted this project (`/trust` once).
- [ ] For Path 2: Hermes/Chief session up, with the `code-editing-agent` workspace routable (workspace-label routing).
- [ ] Zoom the terminal font before starting; the 3-column layout is the money shot.

## Path 1 — direct Claude session

Start `claude` in this repo, then paste:

> Run the agentic workflow for this feature: add a **move_file** tool to the agent, alongside the
> existing three tools. It takes `old_path` and `new_path`, moves/renames the file, creates missing
> parent directories of the destination, and refuses to overwrite an existing destination —
> failures come back to the model as tool errors, not crashes. Keep the design lightweight;
> no design gate, keep the plan approval gate.

Notes:

- The grilling round still runs — answer via the option picker; that *is* part of the demo
  (show that ambiguity is settled up front, by keyboard, not prose ping-pong).
- Expected stops: (a) grill questions, (b) plan gate with visual plan URL, (c) Hunk tab at the end.
- Narrate the pane monitor when a delegate goes `blocked` — "the orchestrator got notified, we
  didn't discover it by staring at panes."

## Path 2 — Hermes/Chief path

Send this to the Hermes (Master Chief) session:

> Create a task for the **code-editing-agent** workspace: launch Claude in
> `~/Documents/Personal/Coding/code-editing-agent` and have it run the repo's orchestrated agentic
> workflow (the `orchestrate` skill) for this brief: add a **delete_file** tool to the agent — takes
> `path`, deletes exactly one regular file, refuses directories and missing paths with a clear tool
> error, and is registered alongside the existing tools. Reporting contract: you are the parent —
> the orchestrator pauses at each approval gate and reports the decision package to you; relay
> gates to me for the actual go/no-go, and bring me the close-out report (PR URL, commits, test
> evidence) when done.

Notes:

- Same orchestrator, same routing matrix — the only difference is the reporting mode: gates pause
  for the **parent's** response instead of a human in-session (`orchestrate` skill, "Reporting mode").
- Talking point: this is how the workflow composes upward — a fleet parent can drive many project
  orchestrators, but git state still belongs to each repo's own orchestrator.

## Why these two tasks

`move_file` and `delete_file` are deliberately medium: real design decisions exist (overwrite
policy, directory refusal, error surface back to the model) so design/review aren't vacuous, but
each is one tool in one file, so the design stage stays minutes, not an hour. Avoid multi-tool asks
(e.g. re-building run_command + search_files) live — the real run of that feature took multiple
design-review rounds.

## Timing (rough)

| Segment | Time |
| --- | --- |
| Grilling | 2–3 min |
| Design + design review loop | 5–10 min |
| Plan + review + visual plan + gate | 5 min |
| Implement + task review + stage test | 10–15 min |
| Final gates + PR + Hunk tab | 5–10 min |

Total ≈ 30–45 min per path. If time is short: run Path 1 live end-to-end, and for Path 2 just show
the launch + first gate reporting to the parent, then cut to a finished run.

## Reset between runs / after the demo

File-scoped only — never `git reset --hard` (it would clobber workflow files like `.claude/skills/`):

```sh
git checkout main
git branch -D <feat-branch>            # discards the run's commits
git checkout 885bc8a -- '*.go'         # restore Go files to the demo baseline, touch nothing else
git status --short                     # any leftover untracked *.go from the run → rm by hand
rm -rf docs/design/<slug> docs/planning/<slug>
```

The full run-search-tools implementation still exists in history (merge commit `ee93b66`) if anyone
wants to see a real, shipped output of this workflow — PR #1.
