package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func swapSeams(t *testing.T, in io.Reader, terminal bool) *bytes.Buffer {
	t.Helper()
	origIn, origOut, origTerm, origTO := confirmIn, confirmOut, stdinIsTerminal, commandTimeout
	out := new(bytes.Buffer)
	confirmIn = in
	confirmOut = out
	stdinIsTerminal = func() bool { return terminal }
	t.Cleanup(func() {
		confirmIn, confirmOut, stdinIsTerminal, commandTimeout = origIn, origOut, origTerm, origTO
	})
	return out
}

func callRunCommand(t *testing.T, command string) (string, error) {
	t.Helper()
	raw, err := json.Marshal(RunCommandInput{Command: command})
	if err != nil {
		t.Fatal(err)
	}
	return RunCommand(raw)
}

func TestRunCommandGate(t *testing.T) {
	tests := []struct {
		name     string
		confirm  string
		terminal bool
		approve  bool
	}{
		{name: "y", confirm: "y\n", terminal: true, approve: true},
		{name: "Y", confirm: "Y\n", terminal: true, approve: true},
		{name: "yes", confirm: "yes\n", terminal: true, approve: true},
		{name: " y ", confirm: " y \n", terminal: true, approve: true},
		{name: "n", confirm: "n\n", terminal: true, approve: false},
		{name: "empty line", confirm: "\n", terminal: true, approve: false},
		{name: "EOF", confirm: "", terminal: true, approve: false},
		{name: "garbage", confirm: "nope\n", terminal: true, approve: false},
		{name: "non-TTY", confirm: "y\n", terminal: false, approve: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader(tc.confirm)
			startLen := r.Len()
			out := swapSeams(t, r, tc.terminal)

			if tc.approve {
				got, err := callRunCommand(t, "echo hi")
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if !strings.Contains(got, "exit code: 0") {
					t.Fatalf("missing exit code: 0 in %q", got)
				}
				if !strings.Contains(got, "hi") {
					t.Fatalf("missing hi in %q", got)
				}
				if !strings.Contains(out.String(), "echo hi") {
					t.Fatalf("prompt missing command: %q", out)
				}
				return
			}

			sentinel := filepath.Join(t.TempDir(), "sentinel")
			cmd := "touch " + sentinel
			_, err := callRunCommand(t, cmd)
			if err == nil {
				t.Fatal("expected error")
			}
			if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
				t.Fatalf("process started; sentinel exists: %v", statErr)
			}
			if !tc.terminal {
				if !strings.Contains(err.Error(), "non-interactive") {
					t.Fatalf("err = %v, want non-interactive", err)
				}
				if out.Len() != 0 {
					t.Fatalf("prompted on non-TTY: %q", out)
				}
				if r.Len() != startLen {
					t.Fatalf("read confirmIn on non-TTY, leftover %d want %d", r.Len(), startLen)
				}
				return
			}
			wantDeclined := "user declined to run this command; do not retry it \u2014 ask the user or try a different approach"
			if err.Error() != wantDeclined {
				t.Fatalf("err = %q, want %q", err.Error(), wantDeclined)
			}
			if !strings.Contains(out.String(), cmd) {
				t.Fatalf("prompt missing command: %q", out)
			}
		})
	}
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestRunCommandReadError(t *testing.T) {
	out := swapSeams(t, errReader{err: errors.New("boom")}, true)
	sentinel := filepath.Join(t.TempDir(), "sentinel")
	cmd := "touch " + sentinel
	_, err := callRunCommand(t, cmd)
	if err == nil {
		t.Fatal("expected error")
	}
	wantDeclined := "user declined to run this command; do not retry it \u2014 ask the user or try a different approach"
	if err.Error() != wantDeclined {
		t.Fatalf("err = %q, want %q", err.Error(), wantDeclined)
	}
	if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
		t.Fatalf("process started; sentinel exists: %v", statErr)
	}
	if !strings.Contains(out.String(), cmd) {
		t.Fatalf("prompt missing command: %q", out)
	}
}

func TestRunCommandLeftover(t *testing.T) {
	r := strings.NewReader("y\nSECOND LINE\n")
	swapSeams(t, r, true)
	if _, err := callRunCommand(t, "echo hi"); err != nil {
		t.Fatal(err)
	}
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != "SECOND LINE\n" {
		t.Fatalf("leftover = %q", rest)
	}
}

func TestRunCommandOutputCap(t *testing.T) {
	swapSeams(t, strings.NewReader("y\n"), true)
	got, err := callRunCommand(t, "head -c 20000 /dev/zero | tr '\\0' a")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("missing [truncated] in %q", got[:min(len(got), 80)])
	}
	_, rest, ok := strings.Cut(got, "\n")
	if !ok {
		t.Fatalf("no newline in %q", got[:min(len(got), 80)])
	}
	output, _, found := strings.Cut(rest, "[truncated]")
	if !found {
		t.Fatal("cut failed")
	}
	output = strings.TrimSuffix(output, "\n")
	if len(output) > maxCommandOutput {
		t.Fatalf("output section %d > %d", len(output), maxCommandOutput)
	}
}

func TestRunCommandExitCode(t *testing.T) {
	swapSeams(t, strings.NewReader("y\n"), true)
	got, err := callRunCommand(t, "exit 3")
	if err != nil {
		t.Fatalf("nonzero exit must be a result, err = %v", err)
	}
	if !strings.Contains(got, "exit code: 3") {
		t.Fatalf("got %q", got)
	}
}

func TestRunCommandTimeout(t *testing.T) {
	swapSeams(t, strings.NewReader("y\n"), true)
	commandTimeout = 100 * time.Millisecond
	start := time.Now()
	got, err := callRunCommand(t, "sleep 5")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("timeout must be a result, err = %v", err)
	}
	if elapsed >= 3*time.Second {
		t.Fatalf("took %s, want < 3s", elapsed)
	}
	if !strings.Contains(got, "timed out after 100ms") {
		t.Fatalf("got %q", got)
	}
}

func TestRunCommandCombinedOutput(t *testing.T) {
	swapSeams(t, strings.NewReader("y\n"), true)
	got, err := callRunCommand(t, "echo out; echo err >&2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "out") || !strings.Contains(got, "err") {
		t.Fatalf("got %q", got)
	}
}

func TestRunCommandPromptEscapesControlBytes(t *testing.T) {
	out := swapSeams(t, strings.NewReader("n\n"), true)
	cmd := "echo hi\x1b[2J\nevil"
	_, err := callRunCommand(t, cmd)
	if err == nil {
		t.Fatal("expected decline")
	}
	got := out.String()
	quoted := fmt.Sprintf("%q", cmd)
	if !strings.Contains(got, quoted) {
		t.Fatalf("prompt missing quoted command %s: %q", quoted, got)
	}
	if strings.Contains(got, "\x1b") {
		t.Fatalf("prompt contains raw ESC: %q", got)
	}
}

func TestRunCommandWaitDelayKeepsOutput(t *testing.T) {
	swapSeams(t, strings.NewReader("y\n"), true)
	got, err := callRunCommand(t, "echo OUT; sleep 30 &")
	if err != nil {
		t.Fatalf("WaitDelay must be a result, err = %v", err)
	}
	if !strings.Contains(got, "OUT") {
		t.Fatalf("discarded output: %q", got)
	}
	if !strings.Contains(got, "background process") {
		t.Fatalf("missing wait-delay note: %q", got)
	}
}

func TestFormatRunResultUTF8Boundary(t *testing.T) {
	output := bytes.Repeat([]byte("a"), maxCommandOutput+2)
	copy(output[maxCommandOutput-1:], []byte("€")) // e2 82 ac straddles the 10240-byte cut
	got := formatRunResult(0, "", output)
	_, rest, ok := strings.Cut(got, "\n")
	if !ok {
		t.Fatal("no newline")
	}
	body, _, found := strings.Cut(rest, "[truncated]")
	if !found {
		t.Fatal("missing [truncated]")
	}
	body = strings.TrimSuffix(body, "\n")
	if !utf8.ValidString(body) {
		t.Fatalf("invalid utf-8 after cap, last bytes %q", body[max(0, len(body)-8):])
	}
}

func TestRunCommandInvalidJSON(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	_, err := RunCommand([]byte(`{`))
	if err == nil {
		t.Fatal("expected error")
	}
}
