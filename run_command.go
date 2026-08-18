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
	fmt.Fprintf(confirmOut, "run_command wants to execute:\n  %s\nAllow? [y/N]: ", command)
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
	exitCode := 0
	if timedOut {
		exitCode = -1
	} else if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			return "", err
		}
	}
	return formatRunResult(exitCode, timedOut, output), nil
}

func formatRunResult(exitCode int, timedOut bool, output []byte) string {
	var b strings.Builder
	if timedOut {
		b.WriteString("exit code: -1 (killed: timed out after 30s)\n")
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
	}
	b.Write(output)
	if truncated {
		if output[len(output)-1] != '\n' {
			b.WriteByte('\n')
		}
		b.WriteString("[truncated]")
	}
	return b.String()
}
