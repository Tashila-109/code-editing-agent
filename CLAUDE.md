# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A minimal code-editing agent: an interactive CLI chat loop that talks to the Claude API (Sonnet 5) and gives the model three tools — `read_file`, `list_files`, `edit_file`. The entire program lives in `main.go` (module name: `agent`). There are no tests.

## Commands

```sh
go build ./...        # compile
go run .              # run the interactive agent (reads from stdin)
go vet ./...          # static checks
```

Running requires `ANTHROPIC_API_KEY` in the environment (picked up automatically by `anthropic.NewClient()`).

## Agentic Workflow

This repo doubles as the lab for the orchestrated multi-CLI workflow. The full protocol — routing matrix, model rule (Fable 5/high orchestrator, Opus 5 designer/planner + final test gate + adversarial final review, Sonnet 5 for other spawned Claude agents), 3-column Herdr layout, a persistent pane monitor, retry→fallback, commit gating, and a focused Hunk-review tab as the final human gate — lives in the `orchestrate` skill; invoke it for any multi-file task. Bugbot is opt-in only. Invariants that hold even outside orchestration:

- Never work on `main` — branch `feat|fix|chore/<slug>` first.
- Only the orchestrator (main session) writes git state and `.claude/{checkpoint,never-again}.md`; read both before starting work.
- Design docs: `docs/design/<slug>/design.md`. Plans: `docs/planning/<slug>/plan.md`.
- Verification is `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...` with verbatim evidence; the final gate adds `-race` and the `go run . < /dev/null` smoke run.

## Architecture

Everything is in `main.go`:

- `Agent.Run` is the core loop: read user input → `runInference` (Messages API call with the tool list) → print text blocks / execute `tool_use` blocks → append tool results as a user message and re-infer without reading new input (`readUserInput` flag) until the model stops calling tools.
- Tools are `ToolDefinition` values (name, description, JSON schema, Go function). To add a tool: define an input struct with `jsonschema_description` tags, generate its schema with `GenerateSchema[T]()`, create a `ToolDefinition`, and add it to the `tools` slice in `main`.
- Tool functions take `json.RawMessage` and return `(string, error)`; errors are sent back to the model as error tool results, not fatal.
- `edit_file` does exact string replacement (all occurrences); an empty `old_str` on a nonexistent path creates the file (including parent dirs).
