package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxMatches   = 100
	maxMatchLine = 250
)

type SearchFilesInput struct {
	Pattern string `json:"pattern" jsonschema_description:"Go (RE2) regular expression matched against each line of file contents."`
	Path    string `json:"path,omitempty" jsonschema_description:"Optional relative directory to search. Defaults to the current directory."`
}

var SearchFilesInputSchema = GenerateSchema[SearchFilesInput]()

var SearchFilesDefinition = ToolDefinition{
	Name:        "search_files",
	Description: "Search file contents for a Go (RE2) regular expression. Skips .git directories and binary files. Returns matching lines as path:line: text. If no path is provided, searches the current directory.",
	InputSchema: SearchFilesInputSchema,
	Function:    SearchFiles,
}

func SearchFiles(input json.RawMessage) (string, error) {
	var in SearchFilesInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	if in.Pattern == "" {
		return "", fmt.Errorf("empty pattern")
	}

	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return "", err
	}

	root := in.Path
	if root == "" {
		root = "."
	}

	var matches []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		sniff := data
		if len(sniff) > 8192 {
			sniff = sniff[:8192]
		}
		if bytes.IndexByte(sniff, 0) >= 0 {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		if n := len(lines); n > 0 && lines[n-1] == "" {
			lines = lines[:n-1]
		}

		for i, line := range lines {
			if !re.MatchString(line) {
				continue
			}
			text := line
			if len(text) > maxMatchLine {
				cut := maxMatchLine
				for cut > 0 && !utf8.RuneStart(text[cut]) {
					cut--
				}
				text = text[:cut] + "…"
			}
			matches = append(matches, fmt.Sprintf("%s:%d: %s", p, i+1, text))
			if len(matches) > maxMatches {
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "no matches", nil
	}
	note := ""
	if len(matches) > maxMatches {
		matches = matches[:maxMatches]
		note = "\n[truncated: first 100 matches shown]"
	}
	return strings.Join(matches, "\n") + note, nil
}
