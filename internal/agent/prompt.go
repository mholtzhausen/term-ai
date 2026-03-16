package agent

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// BuildSystemPrompt wraps base with a context preamble that gives the agent
// situational awareness and agentic reasoning guidelines.  The stored system
// prompt (base) is appended last as the agent's specific role definition.
//
// This function is called at every point where a system message is constructed
// so that ALL execution paths (CLI, TUI, sub-agent delegation) get the prefix.
// The stored value in the database is never modified.
func BuildSystemPrompt(base string) string {
	now := time.Now()
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "(unknown)"
	}

	prefix := fmt.Sprintf(`You are an intelligent, tool-enabled AI agent. Before acting, reason carefully and identify exactly what information you need.

## Agentic Reasoning Guidelines
- **Think systematically**: decompose complex requests into concrete, ordered steps before acting.
- **Think laterally**: consider non-obvious approaches and alternative framings before committing.
- **Augment yourself**: identify knowledge gaps and fill them proactively with tool calls before answering.
- **Verify assumptions**: use tools to confirm facts rather than guessing or hallucinating.
- **Delegate wisely**: use sub-agents for specialised tasks outside your core competency when available.
- **Be concise in tool arguments; be thorough and precise in final answers.**

## Environment
- Date: %s
- Time: %s
- Working Directory: %s
- Platform: %s/%s

## Role
%s`,
		now.Format("Monday, 02 January 2006"),
		now.Format("15:04 MST"),
		cwd,
		runtime.GOOS,
		runtime.GOARCH,
		base,
	)

	return prefix
}
