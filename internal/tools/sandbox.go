package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mhai-org/term-ai/internal/ai"
)

const tmpBase = "/tmp/term-ai"

func init() {
	register("shell_exec", shellExecDef(), runShellExec)
	register("write_tmp", writeTmpDef(), runWriteTmp)
	register("exec_script", execScriptDef(), runExecScript)
}

// sandboxPath resolves relPath inside tmpBase and verifies it stays within that directory.
func sandboxPath(relPath string) (string, error) {
	target := filepath.Clean(filepath.Join(tmpBase, relPath))
	if target != tmpBase && !strings.HasPrefix(target, tmpBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes sandbox (%s is not within %s)", relPath, tmpBase)
	}
	return target, nil
}

// ---- shell_exec ---------------------------------------------------------

func shellExecDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Shell command to execute."}
		},
		"required": ["command"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "shell_exec",
			Description: "Execute a shell command after user confirmation. Shows the command to the user and requires approval before running. Output is capped at 10 KB with a 30-second timeout.",
			Parameters:  params,
		},
	}
}

func runShellExec(argsJSON string) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("shell_exec: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Command) == "" {
		return "", fmt.Errorf("shell_exec: command must not be empty")
	}
	if !ConfirmFunc(fmt.Sprintf("Allow command: %s", args.Command)) {
		return "", fmt.Errorf("shell_exec: user denied execution")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Run() //nolint:errcheck — non-zero exit captured in output

	out := buf.Bytes()
	const maxOut = 10 * 1024
	if len(out) > maxOut {
		out = append(out[:maxOut], []byte("\n... (truncated)")...)
	}
	return string(out), nil
}

// ---- write_tmp ----------------------------------------------------------

func writeTmpDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":    {"type": "string", "description": "File path relative to /tmp/term-ai/ (e.g. 'script.sh' or 'sub/file.txt')."},
			"content": {"type": "string", "description": "Content to write to the file."}
		},
		"required": ["path", "content"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "write_tmp",
			Description: "Write a file inside /tmp/term-ai/ (and subdirectories). Paths are restricted to that directory to prevent unintended writes elsewhere.",
			Parameters:  params,
		},
	}
}

func runWriteTmp(argsJSON string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("write_tmp: invalid args: %w", err)
	}

	target, err := sandboxPath(args.Path)
	if err != nil {
		return "", fmt.Errorf("write_tmp: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", fmt.Errorf("write_tmp: could not create directory: %w", err)
	}
	if err := os.WriteFile(target, []byte(args.Content), 0644); err != nil {
		return "", fmt.Errorf("write_tmp: %w", err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(args.Content), target), nil
}

// ---- exec_script --------------------------------------------------------

func execScriptDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Script path relative to /tmp/term-ai/ (e.g. 'script.sh')."},
			"args": {"type": "array", "items": {"type": "string"}, "description": "Optional arguments to pass to the script."}
		},
		"required": ["path"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "exec_script",
			Description: "Execute a script that exists in /tmp/term-ai/ after user confirmation. The script must already exist (use write_tmp first). Output capped at 10 KB, 30-second timeout.",
			Parameters:  params,
		},
	}
}

func runExecScript(argsJSON string) (string, error) {
	var args struct {
		Path string   `json:"path"`
		Args []string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("exec_script: invalid args: %w", err)
	}

	target, err := sandboxPath(args.Path)
	if err != nil {
		return "", fmt.Errorf("exec_script: %w", err)
	}

	if _, err := os.Stat(target); err != nil {
		return "", fmt.Errorf("exec_script: script not found: %s", target)
	}

	if !ConfirmFunc(fmt.Sprintf("Allow script execution: %s", target)) {
		return "", fmt.Errorf("exec_script: user denied execution")
	}

	// Make executable.
	if err := os.Chmod(target, 0755); err != nil {
		return "", fmt.Errorf("exec_script: could not set executable bit: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "bash", append([]string{target}, args.Args...)...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Run() //nolint:errcheck

	out := buf.Bytes()
	const maxOut = 10 * 1024
	if len(out) > maxOut {
		out = append(out[:maxOut], []byte("\n... (truncated)")...)
	}
	return string(out), nil
}
