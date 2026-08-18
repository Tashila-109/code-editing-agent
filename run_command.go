package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

const maxCommandOutput = 10240

var (
	confirmIn       io.Reader = os.Stdin
	confirmOut      io.Writer = os.Stdout
	commandTimeout            = 30 * time.Second
	stdinIsTerminal           = func() bool {
		fi, err := os.Stdin.Stat()
		if err != nil {
			return false
		}
		return fi.Mode()&os.ModeCharDevice != 0
	}

	errDeclined       = errors.New("user declined to run this command; do not retry it \u2014 ask the user or try a different approach")
	errNonInteractive = errors.New("run_command requires an interactive terminal; declined (non-interactive session) — do not retry")
)

type RunCommandInput struct {
	Command string `json:"command" jsonschema_description:"The shell command to execute via sh -c. The user is shown the command and must approve it before it runs. If the user declines, do not retry the same command."`
}

var RunCommandInputSchema = GenerateSchema[RunCommandInput]()

var RunCommandDefinition = ToolDefinition{
	Name:        "run_command",
	Description: "Execute a shell command via sh -c. The user is shown the exact command and must approve it before it runs. If the user declines, do not retry the same command.",
	InputSchema: RunCommandInputSchema,
	Function:    RunCommand,
}

func RunCommand(input json.RawMessage) (string, error) {
	var in RunCommandInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	if err := confirmRun(in.Command); err != nil {
		return "", err
	}
	return runApproved(in.Command)
}

func confirmRun(command string) error {
	if !stdinIsTerminal() {
		return errNonInteractive
	}
	shown := command
	if commandHasUnsafeDisplayBytes(command) {
		shown = fmt.Sprintf("%q", command)
	}
	fmt.Fprintf(confirmOut, "run_command wants to execute:\n  %s\nAllow? [y/N]: ", shown)
	line, err := readOneLine(confirmIn)
	if err != nil && err != io.EOF {
		return errDeclined
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return errDeclined
	}
}

func commandHasUnsafeDisplayBytes(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c != 0x7f {
			continue
		}
		if c == '\n' && i == len(s)-1 {
			continue
		}
		return true
	}
	return false
}

func readOneLine(r io.Reader) (string, error) {
	var buf []byte
	b := make([]byte, 1)
	for {
		n, err := r.Read(b)
		if n > 0 {
			if b[0] == '\n' {
				return string(buf), nil
			}
			buf = append(buf, b[0])
		}
		if err != nil {
			return string(buf), err
		}
	}
}

func runApproved(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	// ponytail: CommandContext kills sh only; process-group kill if orphaned grandchildren matter
	cmd.WaitDelay = 2 * time.Second

	output, err := cmd.CombinedOutput()
	timedOut := ctx.Err() == context.DeadlineExceeded
	note := ""
	exitCode := 0
	switch {
	case timedOut:
		exitCode = -1
		note = fmt.Sprintf("killed: timed out after %s", commandTimeout)
	case errors.Is(err, exec.ErrWaitDelay):
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		note = "background process still holds the pipes"
	case err != nil:
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return "", err
		}
		exitCode = ee.ExitCode()
	}
	return formatRunResult(exitCode, note, output), nil
}

func formatRunResult(exitCode int, note string, output []byte) string {
	var b strings.Builder
	if note != "" {
		fmt.Fprintf(&b, "exit code: %d (%s)\n", exitCode, note)
	} else {
		fmt.Fprintf(&b, "exit code: %d\n", exitCode)
	}
	if len(output) == 0 {
		b.WriteString("(no output)")
		return b.String()
	}
	truncated := len(output) > maxCommandOutput
	if truncated {
		output = output[:maxCommandOutput]
		for len(output) > 0 {
			r, size := utf8.DecodeLastRune(output)
			if r != utf8.RuneError || size != 1 {
				break
			}
			output = output[:len(output)-1]
		}
	}
	b.Write(output)
	if truncated {
		if len(output) == 0 || output[len(output)-1] != '\n' {
			b.WriteByte('\n')
		}
		b.WriteString("[truncated]")
	}
	return b.String()
}
