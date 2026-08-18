# run_command and search_files tools

> **Status:** Proposed for review

## 1. Executive summary

The agent can read, list, and edit files but cannot run anything or search file contents, so the model works blind: it cannot build the code it just edited or find where a symbol is used. This change adds two tools. `run_command` executes a model-chosen shell command via `sh -c`, but only after the human types `y` at an interactive confirmation prompt (default: no); the gate requires stdin to be a real terminal — in non-interactive sessions every command auto-declines. Output is combined stdout+stderr capped at 10KB, the exit code is always reported, and a fixed 30-second timeout kills runaway commands. `search_files` runs a Go RE2 regex over file contents under an optional path, skipping `.git` and binary files, returning up to 100 `path:line: text` matches. Each tool lives in its own new file with table-driven tests; `main.go` changes only to register them. The main downside: the confirmation prompt shares stdin with the chat loop, and with the "registration-only" constraint on `main.go` the two readers cannot share one buffer — a user who types the `y` answer *ahead of time* (before the prompt appears) will have it stranded in the chat scanner's buffer and must answer again.

## 2. Context and scope

Today `main.go` holds the whole agent: a `bufio.Scanner` over stdin feeds the chat loop, and tools are `ToolDefinition` values with `Function func(json.RawMessage) (string, error)`. Tool functions that return a Go error are sent back to the model as error tool results — though the existing tools do not honor that contract fully: `ReadFile` and `ListFiles` `panic` on malformed JSON input instead of returning an error (a pre-existing crash path, out of scope here). There are no tests in the repo.

This design covers: the two new tools (`run_command.go`, `search_files.go`), their schemas, the confirmation gate's stdin handling, failure behavior back to the model, and their tests. It does not cover changes to the tool machinery, the chat loop, or the existing three tools. All product decisions (sh -c, y/N gate, 10KB cap, 30s timeout, RE2 walk, 100-match cap) arrived settled and are not revisited here — only their mechanics are designed.

## 3. Proposed design

### How it works — one real case

The user asks "does this build?". The model calls `run_command` with `{"command": "go build ./..."}`. The tool first checks that stdin is an interactive terminal (`stdinIsTerminal`, Section 5); if it is not, it returns the non-interactive decline error without prompting. Stdin is a terminal here, so the tool prints to stdout:

```
run_command wants to execute:
  go build ./...
Allow? [y/N]:
```

It then reads **one line** from stdin, byte by byte, directly from `os.Stdin` (no new buffered reader — see Decisions). The user types `y⏎`. The tool runs `exec.CommandContext(ctx, "sh", "-c", "go build ./...")` with a 30s deadline and `WaitDelay` of 2s, collects combined output, and returns:

```
exit code: 0
(no output)
```

The result flows back through the existing `executeTool` path as a normal (non-error) tool result and the model answers the user. Had the user typed anything else — or just Enter — the tool would return the error `user declined to run this command; do not retry it — ask the user or try a different approach`, which `executeTool` already converts into an error tool result.

### Components and responsibilities

**`run_command.go`** — owns: the `run_command` tool definition, schema, and function; the interactive-terminal check; the confirmation prompt and its one-line stdin read; process execution, timeout kill, output capping, exit-code formatting; package-level seams `confirmIn io.Reader = os.Stdin`, `confirmOut io.Writer = os.Stdout`, `commandTimeout = 30 * time.Second`, `stdinIsTerminal func() bool` (vars so tests can substitute). Depends on: `os/exec`, `context`, `io`. Does not own: deciding *whether* the model should run commands (the human gate does), the chat loop's scanner, or any retry logic.

**`search_files.go`** — owns: the `search_files` tool definition, schema, and function; the `filepath.WalkDir` traversal, `.git` and binary skipping, per-line regex matching, the 100-match cap and truncation note. Depends on: `regexp`, `io/fs`, `path/filepath`, `os`, `bytes`. Does not own: shelling out (pure Go by decision), path sandboxing (consistent with the existing tools — see Security).

**`main.go`** — changes only: `ReadFileDefinition, ListFilesDefinition, EditFileDefinition, RunCommandDefinition, SearchFilesDefinition` in the `tools` slice.

**`run_command_test.go`, `search_files_test.go`** — own the table-driven tests (Section 9).

### Decisions

**Confirmation reads stdin unbuffered, one byte at a time, through the first `\n`.** The chat loop already owns a `bufio.Scanner` on `os.Stdin` (a local inside `main()`), and the brief fixes `main.go` to registration-only, so the tool cannot share that scanner's buffer. Creating a *second* `bufio` reader in the tool would be a real bug: `bufio` reads up to 64KB per syscall, so the tool's reader could swallow the user's *next chat message* typed after the `y`, and that message would be silently lost when the tool returned. Instead `confirmRun` loops `Read` on a 1-byte buffer until `\n` or EOF. This consumes exactly one line and can never buffer ahead, so the chat scanner's view of stdin is untouched. Cost: the reverse corner survives — if the user typed the answer *before* the prompt appeared (multi-line paste or type-ahead), the chat scanner may already hold that line in its buffer and the tool's read blocks until the user answers again. That is a rare interactive corner with a visible prompt sitting on screen, no data loss, and re-answering recovers it. Rejected: refactoring `main.go` so the chat loop and the gate share one reader — correct on type-ahead, but breaks the settled registration-only constraint; recorded here so a later change can lift it deliberately.

**The gate requires an interactive terminal; non-TTY stdin declines before prompting.** Piped stdin with pre-seeded `y` lines would otherwise satisfy the "human" gate mechanically — a bypass of the one control this tool has. Detection is stdlib-only: `os.Stdin.Stat()` and `fi.Mode()&os.ModeCharDevice != 0`. Pipes and file redirects are not character devices, so exactly the bypass case (pre-seeded piped input) is refused with a distinct non-interactive decline error. One nuance: `/dev/null` *is* a character device, so `go run . < /dev/null` passes this check — and then immediately hits EOF at the read, which also declines (INV-1). The two layers together fail closed on every non-interactive input. Rejected: `golang.org/x/term.IsTerminal` — a true termios check that would also classify `/dev/null` as non-TTY, but it is a new dependency bought to close a hole the EOF-decline layer already closes.

**Decline and "couldn't run" use the error channel; a non-zero exit does not.** A command that runs and exits 3 is a *result* the model needs (with its output) to react. Only "the command never ran" cases — user declined, `sh` unstartable — return a Go error, which the existing machinery turns into an error tool result. The decline message explicitly says not to retry, because the settled behavior is that the model moves on rather than re-asking.

**Timeout: `exec.CommandContext` + `cmd.WaitDelay = 2s`; the direct child is killed, grandchildren may survive.** `CommandContext` kills `sh` at the deadline. For a single simple command, `sh -c` execs it, so the kill hits the real process. For compound commands, background grandchildren holding the output pipe would normally hang `CombinedOutput`-style collection forever; `WaitDelay` (Go 1.20+, we're on 1.26) force-closes the pipes 2s after the kill so the tool always returns. Rejected: `Setpgid` + kill the process group — kills grandchildren too, but adds platform-specific syscall code to a lab agent; the ceiling (an orphaned background process) is acceptable and noted in the code with the upgrade path.

**Binary detection: NUL byte in the first 8KB.** Files are read whole with `os.ReadFile` and split on `\n`, which also sidesteps `bufio.Scanner`'s 64KB token limit on long lines. A NUL in the first 8KB marks the file binary and it is skipped. Crude but standard (same family as `git`'s heuristic), and wrong only in ways that cost a missed match, never a crash.

**Each matched line is capped at 250 bytes in the output.** The brief caps match *count* at 100, but one minified 500KB line would still blow up the tool result. Lines longer than 250 bytes are cut with a trailing `…`. Worst case output is therefore bounded at roughly 100 × 300 bytes.

## 4. Invariants and requirements

- **INV-1** — No command is executed unless the confirmation read returns an affirmative line — `y` or `yes`, case-insensitive, surrounding whitespace trimmed — after the exact command text was written to `confirmOut`. Empty line, any other text, read error, and EOF all decline.
- **INV-2** — A declined command returns a tool error containing `user declined`, and the process is never started.
- **INV-3** — The confirmation read consumes at most one line of stdin (bytes up to and including the first `\n`) and never reads past it.
- **INV-4** — Every successful `run_command` result states the exit code; combined output beyond 10240 bytes is cut and marked `[truncated]`.
- **INV-5** — A command still running at `commandTimeout` is killed; the result says it timed out; the tool function returns within `commandTimeout` + `WaitDelay` + scheduling slack (observable bound in tests: timeout 100ms → returns well under 3s).
- **INV-6** — `search_files` never descends into `.git` and never returns lines from a file whose first 8KB contains a NUL byte.
- **INV-7** — `search_files` returns at most 100 matches; when the cap is hit, the result ends with a truncation note and the walk stops (via `fs.SkipAll`).
- **INV-8** — An invalid regex or an unreadable root path returns a tool error; an unreadable file or directory *during* the walk is skipped silently and the search continues. Neither new tool can panic on model-chosen input.
- **INV-9** — When `stdinIsTerminal()` reports stdin is not an interactive terminal, `run_command` returns a tool error containing `non-interactive` without prompting, and the process is never started.

## 5. Interfaces and data

Both tools follow the existing pattern exactly: an input struct with `jsonschema_description` tags, `GenerateSchema[T]()`, a `ToolDefinition` value.

```go
type RunCommandInput struct {
    Command string `json:"command" jsonschema_description:"The shell command to execute via sh -c. The user is shown the command and must approve it before it runs. If the user declines, do not retry the same command."`
}

type SearchFilesInput struct {
    Pattern string `json:"pattern" jsonschema_description:"Go (RE2) regular expression matched against each line of file contents."`
    Path    string `json:"path,omitempty" jsonschema_description:"Optional relative directory to search. Defaults to the current directory."`
}
```

`run_command` result format (always):

```
exit code: <n>[ (killed: timed out after 30s)]
<combined output | (no output)>[
[truncated]]
```

`search_files` result: one `path:line: text` per match (paths relative to the searched root, 1-based line numbers), or `no matches` when none; when capped, a final line `[truncated: first 100 matches shown]`.

Package seams in `run_command.go` (test injection, not config): `confirmIn io.Reader`, `confirmOut io.Writer`, `commandTimeout time.Duration`, and `stdinIsTerminal func() bool` (default: `os.Stdin.Stat()` char-device check). Constants: `maxCommandOutput = 10240`, `maxMatches = 100`, `maxMatchLine = 250`.

No new dependencies; stdlib only.

## 6. Failure behavior

Every failure of the two **new** tools returns to the model through the existing `executeTool` error path (error tool result, conversation continues); no failure designed here is fatal to the agent. (The existing tools' panic-on-bad-JSON path is unchanged and out of scope.)

- **Stdin is not an interactive terminal** (piped or redirected input) → distinct error `run_command requires an interactive terminal; declined (non-interactive session) — do not retry`, returned before any prompt is printed. No process started.
- **User declines / presses Enter / types garbage** → error `user declined to run this command; do not retry it — ask the user or try a different approach`. No process started. The model is expected to move on; if it retries anyway, each retry re-prompts the human, who remains in control.
- **EOF on stdin at confirmation time** (a char-device stdin that yields no input — e.g. the `go run . < /dev/null` smoke gate, which passes the char-device check) → decline with the `user declined` error. Never blocks, never loops. Consequence for the smoke run: any `run_command` the model attempts auto-declines — safe and consistent with the gate's fail-closed intent.
- **`sh` cannot start** → error with the exec failure. No result formatting.
- **Command exits non-zero** → *normal* result with the exit code and output; not an error (the model needs the output to react).
- **Timeout** → `sh` killed at 30s, pipes force-closed 2s later, normal result: `exit code: -1 (killed: timed out after 30s)` plus whatever output was collected before the kill. Background grandchildren of compound commands may survive as orphans (accepted ceiling, Section 3).
- **Output over 10KB** → cut at 10240 bytes, `[truncated]` appended. Exit code is still present because it is printed before the output.
- **Invalid regex** → error containing the `regexp.Compile` message so the model can correct the pattern.
- **Nonexistent / unreadable search root** → error from the walk root; unreadable entries mid-walk are skipped and the search continues (a permission-denied subtree must not kill the whole search).
- **Chat-loop type-ahead corner** — user answered before the prompt appeared → the answer sits in the chat scanner's buffer; the gate blocks on a visible `Allow? [y/N]:` prompt until the user answers again. Recoverable in place; the stranded line is later consumed as a chat message, which the user can see and ignore.

## 7. Security

The trust boundary is between the model (which chooses every tool input) and the local machine. `run_command` is arbitrary code execution by design; the *only* enforcement is the human gate, so the gate carries four obligations: stdin must be an interactive terminal (INV-9 — piped input pre-seeded with `y` lines cannot mechanically satisfy the gate; it is refused before the prompt is even printed), the exact command is printed verbatim before the prompt (never truncated or summarized — the human must see precisely what they approve), the default is **no** (any input other than `y`/`yes` declines, including EOF and read errors — fail closed), and approval is per-invocation with no "always allow" state. The residual gap in the char-device check (`/dev/null` and other input-yielding character devices) is closed by the EOF/decline layer: only an interactive device that delivers a literal affirmative line can approve.

`search_files` accepts a model-chosen path and pattern. It does no path sandboxing — deliberately consistent with `read_file`/`list_files`, which already accept any path including absolute ones, and moot next to gated arbitrary execution; the whole agent trusts its operator and their filesystem permissions. RE2 guarantees linear-time matching, so a hostile pattern cannot cause catastrophic backtracking; the match cap, line cap, and binary skip bound the output. Both new tools must survive arbitrary malformed input with an error, never a panic (INV-8); the existing tools' `panic` on unmarshal failure is a pre-existing gap left unchanged (Section 11).

## 8. Acceptance criteria

- **AC-1** — With `confirmIn` = `"y\n"`, `run_command` on `echo hi` returns a non-error result containing `hi` and `exit code: 0`.
- **AC-2** — With `confirmIn` = `"n\n"`, `""` (EOF), `"\n"`, and `"nope\n"`, a command that would create a sentinel file returns an error containing `user declined` and the sentinel file does not exist.
- **AC-3** — With `confirmIn` = `"y\nSECOND LINE\n"`, the confirmation approves and `SECOND LINE` is still unread on `confirmIn` afterwards (INV-3 observable).
- **AC-4** — A command emitting ~20KB returns a result whose output section is ≤ 10240 bytes plus a `[truncated]` marker.
- **AC-5** — `sh -c "exit 3"` (approved) returns a non-error result containing `exit code: 3`.
- **AC-6** — With `commandTimeout` = 100ms, an approved `sleep 5` returns within 3s with a result stating the timeout.
- **AC-7** — A command writing to both stdout and stderr returns both streams in one result.
- **AC-8** — The prompt written to `confirmOut` contains the full verbatim command.
- **AC-9** — In a temp dir with a matching file, `search_files` returns `path:line: text` lines with correct relative path and 1-based line number; with no matching file it returns `no matches` as a non-error result.
- **AC-10** — A match inside `.git/` and a match inside a NUL-containing file are absent from results.
- **AC-11** — A tree with >100 matching lines returns exactly 100 matches plus the truncation note.
- **AC-12** — An invalid pattern (e.g. `"["`) returns an error mentioning the regex problem; a nonexistent root path returns an error. Neither panics.
- **AC-13** — The `path` parameter restricts results to that subtree; omitted, it searches from `.`.
- **AC-14** — Both tools are registered: the `tools` slice in `main.go` has five entries and `go run . < /dev/null` still starts and exits cleanly.
- **AC-15** — With `stdinIsTerminal` returning false, `run_command` returns an error containing `non-interactive`, nothing is written to `confirmOut`, nothing is read from `confirmIn`, and a sentinel file the command would create does not exist.

## 9. Test approach

First tests in the repo; plain `go test`, table-driven, no frameworks, no new dependencies.

**`run_command_test.go`** — each case swaps `confirmIn` for a `strings.Reader`, `confirmOut` for a `bytes.Buffer`, `stdinIsTerminal` for a stub (true in gate tests, false in the non-interactive case), and (where needed) `commandTimeout`, restoring all four via `t.Cleanup`. The gate table drives INV-1/INV-2/INV-9 and AC-1/AC-2/AC-8/AC-15 (affirmative variants `y`, `Y`, `yes`, ` y `; decline variants `n`, empty, EOF, garbage, non-TTY — sentinel file via `t.TempDir()` proves "never started"). AC-3 proves INV-3 by reading the leftover from the shared reader after the call. Separate cases cover AC-4 (cap, via `head -c 20000 /dev/zero | tr '\0' a`), AC-5 (exit code path, INV-4), AC-6 (timeout with 100ms override and an elapsed-time assertion, INV-5), AC-7 (combined streams). These tests execute real `sh` — that is the unit under test, and CI/dev is darwin/linux.

**`search_files_test.go`** — one `t.TempDir()` fixture per table (plain matching file, nested dir, `.git/config` with a match, file with NUL bytes and a match, 150-match file) drives INV-6/INV-7/INV-8 and AC-9…AC-13. Regex/path error cases assert error content and, implicitly, no panic.

**AC-14** is proved by the repo's standard gate (`go build`, `go vet`, `go test`, smoke run) — the smoke run doubles as the live fail-closed check: `/dev/null` passes the char-device check but any prompt immediately hits EOF and declines.

## 10. Risks and open questions

**Risks**

- Type-ahead answers strand in the chat scanner's buffer → gate blocks until re-answered. Mitigation: visible prompt, documented corner, recoverable in place; real fix (shared reader) recorded in Decisions for a future change.
- Orphaned grandchildren after timeout of compound commands. Mitigation: `WaitDelay` guarantees the tool returns; process-group kill is the noted upgrade path.
- Model retries a declined command with cosmetic changes. Mitigation: every attempt re-prompts the human; the decline message instructs the model not to retry; the tool description repeats it.
- The TTY requirement means a scripted (piped-stdin) session can never approve a command — `run_command` is unusable there. Accepted: that is the point of a human gate; the distinct non-interactive error tells the model to stop trying, and the other four tools still work.
- `search_files` on a huge tree reads every non-binary file fully. Mitigation: match cap stops the walk early via `fs.SkipAll`; acceptable for a lab repo, per-file size skip is the upgrade path if it ever bites.

**Open questions** (none block planning)

- Should decline feed the model the human's reason (e.g. read a rejection note)? Blocks planning: no. Recommended default: no — one-line y/N keeps the gate simple; a reason channel is a feature request, not a gap.

## 11. Out of scope

- Any refactor of the tool machinery, chat loop, or existing tools (including their `panic`-on-unmarshal behavior).
- Path sandboxing / filesystem confinement for any tool.
- Allowlists, "always allow", or per-command approval memory for `run_command`.
- Process-group kill of grandchildren; per-file size limits in search; `.gitignore` awareness.
- Windows support (repo targets darwin/linux; `sh -c` assumed present).
