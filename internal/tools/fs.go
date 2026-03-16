package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mhai-org/term-ai/internal/ai"
)

func init() {
	register("list_files", listFilesDef(), runListFiles)
	register("search_in_files", searchInFilesDef(), runSearchInFiles)
}

// isHiddenName reports whether a single filename component is a hidden (dot) entry.
func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

// isHiddenPath reports whether any component of path is a hidden (dot) name.
func isHiddenPath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	for _, part := range strings.Split(clean, "/") {
		if isHiddenName(part) {
			return true
		}
	}
	return false
}

// ---- list_files ---------------------------------------------------------

func listFilesDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":      {"type": "string",  "description": "Root directory to list (default: current directory)."},
			"recursive": {"type": "boolean", "description": "Descend into subdirectories (default: false)."},
			"filter":    {"type": "string",  "description": "Glob pattern to match filenames against (e.g. '*.go'). Applied to the base filename only. Default: '*' (all files)."}
		}
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "list_files",
			Description: "List files in a directory, ignoring hidden files and directories (names starting with '.'). Supports recursive traversal and an optional glob filter on filenames.",
			Parameters:  params,
		},
	}
}

func runListFiles(argsJSON string) (string, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
		Filter    string `json:"filter"`
	}
	args.Path = "."
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("list_files: invalid args: %w", err)
	}
	if args.Path == "" {
		args.Path = "."
	}
	if args.Filter == "" {
		args.Filter = "*"
	}

	// Validate the glob pattern up-front so we report the error clearly.
	if _, err := filepath.Match(args.Filter, ""); err != nil {
		return "", fmt.Errorf("list_files: invalid filter pattern: %w", err)
	}

	var results []string
	err := filepath.WalkDir(args.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if isHiddenName(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path == args.Path {
				return nil // root: always descend at least one level
			}
			if args.Recursive {
				return nil // continue descending; don't emit the dir itself
			}
			// Non-recursive: emit as a directory entry and skip its contents.
			results = append(results, path+"/")
			return filepath.SkipDir
		}

		// Apply glob filter to the base filename.
		matched, _ := filepath.Match(args.Filter, d.Name())
		if !matched {
			return nil
		}
		results = append(results, path)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "(no files found)", nil
	}
	return strings.Join(results, "\n"), nil
}

// ---- search_in_files ----------------------------------------------------

func searchInFilesDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":    {"type": "string", "description": "Directory to search in (default: current directory)."},
			"pattern": {"type": "string", "description": "Regular expression pattern to search for in file contents."}
		},
		"required": ["pattern"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "search_in_files",
			Description: "Search file contents for a regex pattern, recursively, ignoring hidden files and directories. Returns up to 100 matching lines as 'file:line: content'.",
			Parameters:  params,
		},
	}
}

func runSearchInFiles(argsJSON string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
	}
	args.Path = "."
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("search_in_files: invalid args: %w", err)
	}
	if args.Pattern == "" {
		return "", fmt.Errorf("search_in_files: pattern must not be empty")
	}

	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return "", fmt.Errorf("search_in_files: invalid pattern: %w", err)
	}

	const maxResults = 100
	var results []string

	walkErr := filepath.WalkDir(args.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || len(results) >= maxResults {
			return nil
		}
		if isHiddenName(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		// Skip binary files (contain null bytes).
		if bytes.IndexByte(data, 0) != -1 {
			return nil
		}

		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d: %s", path, i+1, line))
				if len(results) >= maxResults {
					break
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	if len(results) == 0 {
		return "(no matches found)", nil
	}
	out := strings.Join(results, "\n")
	if len(results) >= maxResults {
		out += fmt.Sprintf("\n... (truncated at %d results)", maxResults)
	}
	return out, nil
}
