# Never again

<!-- Orchestrator-only, append-only. One entry per real mistake (not routine friction),
sourced from any role report's ## Learnings or the orchestrator's own observation.

Template:
## YYYY-MM-DD — <incident, one line>
- Root cause:
- Prevention:
-->

## 2026-08-19 — Two Must-fix bugs cleared codex task+branch review and the first Opus gate; only the adversarial fallback caught them
- Root cause: verdict-style reviews and pass/fail tests confirm the code does what it says; they do not try to break it. The confirmation gate printed the model-chosen command verbatim (control bytes could rewrite what the human approved) and search_files ran os.ReadFile on any non-dir entry (a FIFO wedged the agent forever, a device OOM'd, a symlinked file escaped the subtree) — both invisible to reviewers checking the happy path.
- Prevention: keep a dedicated adversarial break-it review in the pipeline even when Bugbot is present, not only as the Bugbot-absent fallback. It reproduced every finding with runnable probes; that is the bar.

## 2026-08-19 — Design stated guarantees as outcomes but the implementation only met the mechanism
- Root cause: design §7 promised the human "sees precisely what they approve" (an outcome) while the code guaranteed byte-fidelity (a mechanism); §3 promised WaitDelay meant "the tool always returns a result" but the error-classification arm for exec.ErrWaitDelay was never written. Reviews checked the mechanism the design named, not the outcome it promised.
- Prevention: when a design states a guarantee as an outcome (the human sees X, the tool always returns Y), the reviewer must verify the outcome, not the named mechanism.

## 2026-08-19 — Any tool that touches a model-chosen filesystem path needs a regular-file guard
- Root cause: os.ReadFile / open on a FIFO or device is an unrecoverable hang or unbounded read in a single-threaded agent loop, and it is invisible in tests because temp dirs only ever contain regular files. The obvious TTY check (os.Stdin.Stat + ModeCharDevice) also classifies /dev/null as a terminal — the run_command gate only survives because an EOF-decline layer sits behind it.
- Prevention: gate model-chosen paths with d.Type().IsRegular() before reading; never rely on ModeCharDevice alone as a safety gate.
