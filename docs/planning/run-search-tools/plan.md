# Plan: run_command and search_files tools

- **Design:** `docs/design/run-search-tools/design.md` (approved; all INV-n / AC-n IDs cited below refer to it)
- **Branch:** `feat/run-search-tools`

| Stage | Tasks | Parallel |
|-------|-------|----------|
| 1 | Add the run_command tool with its human confirmation gate · Add the search_files tool | yes |
| 2 | Register both new tools in main.go | no |

Stage 1's two tasks have disjoint file sets (each creates only its own tool file and test file) and neither consumes anything the other defines — both use only symbols that already exist in `main.go`. Neither task edits `main.go`; the registration edit is the whole of stage 2, so the shared file is touched by exactly one task. Stage 2 starts only after both stage-1 tasks are committed.

---

## Add the run_command tool with its human confirmation gate
Stage: 1 · Parallel-safe: yes

### Summary
The agent (a CLI chat loop in this repo that gives a Claude model file tools) can read, list, and edit files but cannot execute anything, so the model cannot build or test the code it edits. This task adds `run_command`: the model supplies a shell command, the tool prints it verbatim and asks the human `Allow? [y/N]` on the terminal, and only an interactive affirmative answer lets it run via `sh -c` with a 30-second timeout and a 10KB output cap. The tool is complete and tested at package level after this task; it is wired into the agent in stage 2.

### File set (REQUIRED)
- `run_command.go` (new)
- `run_command_test.go` (new — first tests in the repo)

Do **not** edit `main.go` or any existing file.

### Depends on
None.

### Context
Everything lives in package `main`. The existing tool pattern in `main.go` (follow it exactly):
- An input struct with `json` + `jsonschema_description` tags (see `ReadFileInput`, `main.go:152`).
- A schema var via `GenerateSchema[T]()` (`main.go:172`).
- A `ToolDefinition` value (`main.go:138`) with `Name`, `Description`, `InputSchema`, and `Function func(json.RawMessage) (string, error)`.
- A returned Go error becomes an *error tool result* sent back to the model by `executeTool` (`main.go:110`) — the conversation continues; errors are never fatal.

The chat loop owns a `bufio.Scanner` on `os.Stdin` (`main.go:20`). This is why the confirmation gate must never create its own buffered reader on stdin: `bufio` reads ahead up to 64KB and would swallow the user's next chat message. The full mechanics, decisions, and rejected alternatives are in the design, Sections 3, 5, 6, 7 — read it before coding.

What to build (design Section 3/5):
- `RunCommandInput` with one field `Command string` — schema description text verbatim from design Section 5, including the "If the user declines, do not retry the same command" sentence.
- `RunCommandDefinition ToolDefinition`; the tool `Description` must also tell the model that the user sees and must approve every command, and not to retry a declined one.
- Package-level seams (vars, so tests can substitute): `confirmIn io.Reader = os.Stdin`, `confirmOut io.Writer = os.Stdout`, `commandTimeout = 30 * time.Second`, `stdinIsTerminal func() bool` defaulting to an `os.Stdin.Stat()` char-device check (`fi.Mode()&os.ModeCharDevice != 0`). Constant `maxCommandOutput = 10240`.
- Flow: if `stdinIsTerminal()` is false → return the non-interactive error before printing anything. Otherwise print to `confirmOut` the prompt containing the full verbatim command (format in design Section 3), then read **one line** from `confirmIn` by looping `Read` on a 1-byte buffer until `\n` or EOF — never a `bufio` reader. Trim whitespace; `y`/`yes` case-insensitive approves; anything else (empty, garbage, EOF, read error) declines.
- Approved: `exec.CommandContext(ctx, "sh", "-c", cmd)` with a `context.WithTimeout(commandTimeout)` and `cmd.WaitDelay = 2 * time.Second`; collect combined stdout+stderr; format the result exactly as design Section 5 (`exit code: <n>`, timeout marker, `(no output)`, `[truncated]` after 10240 bytes).

### Constraints
- Exact error strings (design Section 6): decline → `user declined to run this command; do not retry it — ask the user or try a different approach`; non-TTY → `run_command requires an interactive terminal; declined (non-interactive session) — do not retry`.
- Decline and "sh could not start" use the Go-error channel; a non-zero exit is a *normal* result with `exit code: <n>` and output (design Decisions).
- Timeout result: `exit code: -1 (killed: timed out after 30s)` plus collected output; grandchild orphans are an accepted ceiling — mark it with a `ponytail:`-style comment naming process-group kill as the upgrade path, per the design.
- No new dependencies (stdlib only; specifically no `golang.org/x/term`). No approval memory or allowlists. No panic on any model-chosen input (INV-8).
- Tests use the seams with `t.Cleanup` restores; they execute real `sh` (dev/CI is darwin/linux — that is the unit under test).

### Acceptance criteria
INV-1, INV-2, INV-3, INV-4, INV-5, INV-9, and the run_command half of INV-8, exactly as written in design Section 4. AC-1 through AC-8 and AC-15, exactly as written in design Section 8, all proven by table-driven tests in `run_command_test.go` per design Section 9 (gate table: affirmative variants `y`, `Y`, `yes`, ` y `; decline variants `n`, empty line, EOF, garbage, non-TTY stub; sentinel file in `t.TempDir()` proves the process never started; AC-3 proves INV-3 by reading the leftover byte stream after the call; AC-6 uses a 100ms `commandTimeout` override with an elapsed-time assertion).

### Checks
```sh
gofmt -l .          # must print nothing
go build ./...
go vet ./...
go test ./...
```

### Out of scope
Editing `main.go` (registration is stage 2). The `search_files` tool. Process-group kill, allowlists, "always allow" state, Windows support, any change to the chat loop or existing tools (including their panic-on-unmarshal path).

---

## Add the search_files tool
Stage: 1 · Parallel-safe: yes

### Summary
The agent cannot search file contents, so the model must read files one by one to find a symbol. This task adds `search_files`: a Go RE2 regex is matched against every line of every text file under an optional relative path, skipping `.git` and binary files, returning up to 100 `path:line: text` matches. Pure Go, no shelling out. Complete and tested at package level after this task; wired into the agent in stage 2.

### File set (REQUIRED)
- `search_files.go` (new)
- `search_files_test.go` (new)

Do **not** edit `main.go` or any existing file.

### Depends on
None.

### Context
Everything lives in package `main`. Follow the existing tool pattern in `main.go`: input struct with `json` + `jsonschema_description` tags (see `ListFilesInput`, `main.go:193` for the optional-path precedent), `GenerateSchema[T]()` (`main.go:172`), a `ToolDefinition` value (`main.go:138`) whose `Function func(json.RawMessage) (string, error)` returns Go errors that `executeTool` (`main.go:110`) converts to error tool results. Full mechanics in the design, Sections 3, 5, 6, 7.

What to build (design Section 3/5):
- `SearchFilesInput` with `Pattern string` and `Path string` (optional) — schema description text verbatim from design Section 5.
- `SearchFilesDefinition ToolDefinition`.
- Compile the pattern with `regexp.Compile`; invalid pattern → error containing the compile message. Root defaults to `.` when `Path` is empty; a nonexistent/unreadable root → error.
- `filepath.WalkDir` from the root: skip `.git` directories entirely (`fs.SkipDir`); skip unreadable files/dirs mid-walk silently and continue. Read each file whole with `os.ReadFile`, split on `\n` (this sidesteps `bufio.Scanner`'s 64KB line limit); a NUL byte in the first 8KB marks the file binary — skip it.
- Emit one `path:line: text` per matching line — path relative to the searched root, 1-based line number; cap each emitted line at 250 bytes with a trailing `…`; stop the walk at 100 matches via `fs.SkipAll` and append `[truncated: first 100 matches shown]`. Zero matches → the non-error result `no matches`.
- Constants: `maxMatches = 100`, `maxMatchLine = 250`.

### Constraints
- Pure Go — no shelling out to grep. Stdlib only. No path sandboxing (deliberately consistent with `read_file`/`list_files`; design Section 7).
- Result and truncation formats exactly as design Section 5.
- No panic on any model-chosen input (INV-8). Errors only for invalid regex and unreadable root; everything mid-walk degrades silently.

### Acceptance criteria
INV-6, INV-7, and the search_files half of INV-8, exactly as written in design Section 4. AC-9 through AC-13, exactly as written in design Section 8, proven by table-driven tests in `search_files_test.go` per design Section 9: `t.TempDir()` fixtures with a plain matching file, a nested directory, a `.git/config` containing a match, a NUL-containing file with a match, and a 150-match file; error cases assert message content and, implicitly, no panic.

### Checks
```sh
gofmt -l .          # must print nothing
go build ./...
go vet ./...
go test ./...
```

### Out of scope
Editing `main.go` (registration is stage 2). The `run_command` tool. Per-file size limits, `.gitignore` awareness, any change to the existing tools.

---

## Register both new tools in main.go
Stage: 2 · Parallel-safe: no

### Summary
After stage 1 both tools exist and pass their tests, but the agent never offers them to the model: the `tools` slice in `main()` still lists only the original three. This task makes the single permitted `main.go` change — adding the two new definitions to that slice — so the model can actually call `run_command` and `search_files`.

### File set (REQUIRED)
- `main.go` (edit — the `tools` slice at `main.go:28` only)

### Depends on
- Add the run_command tool with its human confirmation gate
- Add the search_files tool

### Context
`main.go:28` currently reads:
```go
tools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, EditFileDefinition}
```
Stage 1 created `RunCommandDefinition` (in `run_command.go`) and `SearchFilesDefinition` (in `search_files.go`), both package-level `ToolDefinition` vars in package `main`. Append them to the slice, preserving the existing order and adding the two new entries at the end. That is the entire diff; the design (`docs/design/run-search-tools/design.md`, Section 3) fixes `main.go` to registration-only for this feature.

### Constraints
- No other change to `main.go` — no refactors, no formatting sweeps, no touching the chat loop or existing tools.

### Acceptance criteria
AC-14 exactly as written in design Section 8: the `tools` slice has five entries, and `go run . < /dev/null` starts and exits cleanly. The smoke run doubles as the live fail-closed check for the gate — `/dev/null` is a character device, so any confirmation read would hit EOF and decline (design Section 6); with stdin at EOF the chat loop exits before any API call, so no `ANTHROPIC_API_KEY` is needed.

### Checks
```sh
gofmt -l .              # must print nothing
go build ./...
go vet ./...
go test ./...
go run . < /dev/null    # prints the banner and exits 0
```

### Out of scope
Any behavior change to either tool (fixes go back to the owning stage-1 task's file set). Any other `main.go` edit.
