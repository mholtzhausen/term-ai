package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/mhai-org/term-ai/internal/ai"
)

type toolEntry struct {
	schema  ai.ToolDefinition
	execute func(argsJSON string) (string, error)
}

var registry = map[string]toolEntry{
	"bash":       {schema: bashDef(), execute: runBash},
	"read_file":  {schema: readFileDef(), execute: runReadFile},
	"write_file": {schema: writeFileDef(), execute: runWriteFile},
}

// GetSchemas returns OpenAI tool definitions for the named tools.
// Unknown names are silently skipped.
func GetSchemas(names []string) []ai.ToolDefinition {
	var out []ai.ToolDefinition
	for _, name := range names {
		if t, ok := registry[name]; ok {
			out = append(out, t.schema)
		}
	}
	return out
}

// Execute runs the named tool with the given JSON argument string.
func Execute(name, argsJSON string) (string, error) {
	t, ok := registry[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return t.execute(argsJSON)
}

// Available returns sorted names of all registered tools.
func Available() []string {
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ParseTools splits a comma-separated string of tool names, keeping only
// names that exist in the registry.
func ParseTools(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if _, ok := registry[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

// ---- bash ---------------------------------------------------------------

func bashDef() ai.ToolDefinition {
	params := json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"The shell command to execute."}},"required":["command"]}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "bash",
			Description: "Execute a shell command and return stdout and stderr (max 10 KB output, 15-second timeout).",
			Parameters:  params,
		},
	}
}

func runBash(argsJSON string) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("bash: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Command) == "" {
		return "", fmt.Errorf("bash: command must not be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Run() // non-fatal: capture non-zero exit as output

	out := buf.Bytes()
	const maxOut = 10 * 1024
	if len(out) > maxOut {
		out = append(out[:maxOut], []byte("\n... (truncated)")...)
	}
	return string(out), nil
}

// ---- read_file ----------------------------------------------------------

func readFileDef() ai.ToolDefinition {
	params := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Absolute or relative file path to read."}},"required":["path"]}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "read_file",
			Description: "Read the contents of a file and return them as a string (max 50 KB).",
			Parameters:  params,
		},
	}
}

func runReadFile(argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("read_file: invalid args: %w", err)
	}
	data, err := os.ReadFile(args.Path)
	if err != nil {
		return "", err
	}
	const maxOut = 50 * 1024
	if len(data) > maxOut {
		data = append(data[:maxOut], []byte("\n... (truncated)")...)
	}
	return string(data), nil
}

// ---- write_file ---------------------------------------------------------

func writeFileDef() ai.ToolDefinition {
	params := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path to write."},"content":{"type":"string","description":"Content to write to the file."}},"required":["path","content"]}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "write_file",
			Description: "Write content to a file. Creates the file if it does not exist; overwrites if it does.",
			Parameters:  params,
		},
	}
}

func runWriteFile(argsJSON string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("write_file: invalid args: %w", err)
	}
	if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path), nil
}
