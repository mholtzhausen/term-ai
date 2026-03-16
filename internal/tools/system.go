package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/mhai-org/term-ai/internal/ai"
)

func init() {
	register("system_info", systemInfoDef(), runSystemInfo)
	register("network_info", networkInfoDef(), runNetworkInfo)
	register("process_info", processInfoDef(), runProcessInfo)
}

// runCmd executes a command with a 10-second timeout and returns combined output.
func runCmd(args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Run() //nolint:errcheck — non-zero exit is still useful output
	return buf.String()
}

// ---- system_info --------------------------------------------------------

func systemInfoDef() ai.ToolDefinition {
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "system_info",
			Description: "Return CPU, memory, and disk usage information for the local machine.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
}

func runSystemInfo(_ string) (string, error) {
	cpu := runCmd("sh", "-c", `lscpu | grep -E "^Architecture|^CPU\(s\)|^Model name|^CPU MHz"`)
	mem := runCmd("free", "-h")
	disk := runCmd("df", "-h", "-x", "tmpfs", "-x", "devtmpfs")
	return fmt.Sprintf("=== CPU ===\n%s\n=== Memory ===\n%s\n=== Disk ===\n%s", cpu, mem, disk), nil
}

// ---- network_info -------------------------------------------------------

func networkInfoDef() ai.ToolDefinition {
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "network_info",
			Description: "Return network interface addresses and listening TCP/UDP ports.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
}

func runNetworkInfo(_ string) (string, error) {
	addrs := runCmd("ip", "-br", "addr")
	ports := runCmd("ss", "-tlnp")
	return fmt.Sprintf("=== Addresses ===\n%s\n=== Listening Ports ===\n%s", addrs, ports), nil
}

// ---- process_info -------------------------------------------------------

func processInfoDef() ai.ToolDefinition {
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "process_info",
			Description: "Return the top 20 running processes sorted by CPU usage with their resource consumption.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
}

func runProcessInfo(_ string) (string, error) {
	out := runCmd("ps", "aux", "--sort=-%cpu")
	// Keep header + first 20 data lines.
	lines := splitLines(out)
	if len(lines) > 21 {
		lines = lines[:21]
	}
	if len(lines) == 0 {
		return "(no process information available)", nil
	}
	result := joinLines(lines)
	return result, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
