package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSearchFiles(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) SearchFilesInput
		err   string
		check func(t *testing.T, got string)
	}{
		{
			name: "matching file returns path:line: text",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				writeFile(t, dir, "hello.txt", "aaa\nfoo bar\nbbb\n")
				return SearchFilesInput{Pattern: "foo", Path: dir}
			},
			check: func(t *testing.T, got string) {
				want := "hello.txt:2: foo bar"
				if got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			},
		},
		{
			name: "no matching file returns no matches",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				writeFile(t, dir, "hello.txt", "nothing here\n")
				return SearchFilesInput{Pattern: "foo", Path: dir}
			},
			check: func(t *testing.T, got string) {
				if got != "no matches" {
					t.Fatalf("got %q, want %q", got, "no matches")
				}
			},
		},
		{
			name: "nested directory match",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				writeFile(t, dir, filepath.Join("nested", "dir", "file.txt"), "zzz\nfindme\n")
				return SearchFilesInput{Pattern: "findme", Path: dir}
			},
			check: func(t *testing.T, got string) {
				want := filepath.Join("nested", "dir", "file.txt") + ":2: findme"
				if got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			},
		},
		{
			name: "skips .git and binary files",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				writeFile(t, dir, "visible.txt", "alpha\n")
				writeFile(t, dir, filepath.Join(".git", "config"), "alpha\n")
				writeFile(t, dir, "binary.bin", "alpha\x00still-alpha")
				return SearchFilesInput{Pattern: "alpha", Path: dir}
			},
			check: func(t *testing.T, got string) {
				if got != "visible.txt:1: alpha" {
					t.Fatalf("got %q, want only visible.txt match", got)
				}
				if strings.Contains(got, ".git") {
					t.Fatalf("result leaked .git match: %q", got)
				}
				if strings.Contains(got, "binary.bin") {
					t.Fatalf("result leaked binary match: %q", got)
				}
			},
		},
		{
			name: "caps at 100 matches with truncation note",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				var b strings.Builder
				for i := 0; i < 150; i++ {
					b.WriteString("hit\n")
				}
				writeFile(t, dir, "many.txt", b.String())
				return SearchFilesInput{Pattern: "hit", Path: dir}
			},
			check: func(t *testing.T, got string) {
				lines := strings.Split(got, "\n")
				if len(lines) != 101 {
					t.Fatalf("got %d lines, want 101 (100 matches + note)", len(lines))
				}
				if lines[len(lines)-1] != "[truncated: first 100 matches shown]" {
					t.Fatalf("missing truncation note, last line %q", lines[len(lines)-1])
				}
				for i := 0; i < 100; i++ {
					want := "many.txt:" + strconv.Itoa(i+1) + ": hit"
					if lines[i] != want {
						t.Fatalf("line %d: got %q, want %q", i, lines[i], want)
					}
				}
			},
		},
		{
			name: "invalid pattern",
			setup: func(t *testing.T) SearchFilesInput {
				return SearchFilesInput{Pattern: "[", Path: t.TempDir()}
			},
			err: "regexp",
		},
		{
			name: "nonexistent root",
			setup: func(t *testing.T) SearchFilesInput {
				return SearchFilesInput{Pattern: "foo", Path: filepath.Join(t.TempDir(), "missing")}
			},
			err: "missing",
		},
		{
			name: "path restricts to subtree",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				writeFile(t, dir, filepath.Join("a", "hit.txt"), "needle\n")
				writeFile(t, dir, filepath.Join("b", "hit.txt"), "needle\n")
				return SearchFilesInput{Pattern: "needle", Path: filepath.Join(dir, "a")}
			},
			check: func(t *testing.T, got string) {
				if got != "hit.txt:1: needle" {
					t.Fatalf("got %q, want hit.txt only", got)
				}
				if strings.Contains(got, "b"+string(os.PathSeparator)) || strings.Contains(got, "b/hit") {
					t.Fatalf("result leaked sibling tree: %q", got)
				}
			},
		},
		{
			name: "NUL in first 8KB skipped; NUL after 8KB is searched",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				writeFile(t, dir, "in8k.bin", strings.Repeat("x", 8191)+"\x00FINDME\n")
				writeFile(t, dir, "after8k.txt", strings.Repeat("x", 8192)+"\x00\nFINDME\n")
				return SearchFilesInput{Pattern: "FINDME", Path: dir}
			},
			check: func(t *testing.T, got string) {
				if strings.Contains(got, "in8k.bin") {
					t.Fatalf("NUL in first 8KB should skip: %q", got)
				}
				if !strings.Contains(got, "after8k.txt:2: FINDME") {
					t.Fatalf("NUL after 8KB should still match: %q", got)
				}
			},
		},
		{
			name: "caps emitted line at 250 bytes",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				writeFile(t, dir, "long.txt", strings.Repeat("a", 300)+"\n")
				return SearchFilesInput{Pattern: "aaa", Path: dir}
			},
			check: func(t *testing.T, got string) {
				_, text, ok := strings.Cut(got, ":1: ")
				if !ok {
					t.Fatalf("got %q", got)
				}
				if !strings.HasSuffix(text, "…") {
					t.Fatalf("missing ellipsis: %q", text)
				}
				if len(strings.TrimSuffix(text, "…")) != 250 {
					t.Fatalf("capped text len %d, want 250", len(strings.TrimSuffix(text, "…")))
				}
			},
		},
		{
			name: "unreadable file and dir skipped, search continues",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				writeFile(t, dir, "ok.txt", "needle\n")
				writeFile(t, dir, "secret.txt", "needle\n")
				writeFile(t, dir, filepath.Join("locked", "hidden.txt"), "needle\n")
				secret := filepath.Join(dir, "secret.txt")
				locked := filepath.Join(dir, "locked")
				if err := os.Chmod(secret, 0); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(locked, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_ = os.Chmod(secret, 0644)
					_ = os.Chmod(locked, 0755)
				})
				if _, err := os.ReadFile(secret); err == nil {
					t.Skip("chmod 0 still readable")
				}
				return SearchFilesInput{Pattern: "needle", Path: dir}
			},
			check: func(t *testing.T, got string) {
				if got != "ok.txt:1: needle" {
					t.Fatalf("got %q, want only ok.txt", got)
				}
			},
		},
		{
			name: "omitted path searches from .",
			setup: func(t *testing.T) SearchFilesInput {
				dir := t.TempDir()
				writeFile(t, dir, "here.txt", "cwdmatch\n")
				t.Chdir(dir)
				return SearchFilesInput{Pattern: "cwdmatch"}
			},
			check: func(t *testing.T, got string) {
				if got != "here.txt:1: cwdmatch" {
					t.Fatalf("got %q, want here.txt match from .", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.setup(t)
			got, err := SearchFiles(mustJSON(t, in))
			if tt.err != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error %q, want substring %q", err.Error(), tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, got)
		})
	}
}

func TestSearchFilesMalformedJSON(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked on malformed input: %v", r)
		}
	}()
	_, err := SearchFiles(json.RawMessage(`{`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, in SearchFilesInput) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
