package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mhai-org/term-ai/internal/ai"
	"github.com/mhai-org/term-ai/internal/db"
	"github.com/mhai-org/term-ai/internal/memory"
	"github.com/mhai-org/term-ai/internal/tools"
)

const (
	maxIterations    = 15
	maxSubAgentDepth = 2
)

// Runner executes the agentic ReAct loop for a single session. One Runner is
// created per top-level request; sub-agents are spawned by incrementing depth.
type Runner struct {
	ApiUrl string
	ApiKey string
	Model  string

	// Memory is the shared in-process key-value store for the session.
	// The same instance is passed to all sub-agents so facts persist across
	// delegation boundaries within one session.
	Memory *memory.Memory

	// Callbacks invoked before/after each tool execution (may be nil).
	// These are NOT forwarded to sub-agents so the caller can distinguish
	// which level of the stack produced the event.
	OnToolCall   func(name, args string)
	OnToolResult func(name, result string)

	// depth tracks how many levels of sub-agent delegation we are inside.
	// The root runner starts at 0; each delegate_to_agent call increments it.
	depth int
}

// Run executes the ReAct loop. It calls the LLM, dispatches tool calls,
// appends observations, and iterates until the model emits a final answer or
// maxIterations is reached.
//
// toolNames lists tools from the global registry the agent may use.
// Memory and delegation tools are always appended automatically.
//
// The final assistant text is written to out. The returned []ai.Message is
// the full updated conversation history (includes all tool turns).
func (r *Runner) Run(messages []ai.Message, toolNames []string, out io.Writer) ([]ai.Message, error) {
	schemas := r.buildSchemas(toolNames)

	for i := 0; i < maxIterations; i++ {
		assistantMsg, finishReason, err := ai.StreamChatWithHistoryAndTools(r.ApiUrl, r.ApiKey, r.Model, messages, schemas, out)
		if err != nil {
			return messages, fmt.Errorf("llm call failed: %w", err)
		}

		if finishReason == "tool_calls" || len(assistantMsg.ToolCalls) > 0 {
			messages = append(messages, assistantMsg)

			for _, tc := range assistantMsg.ToolCalls {
				if r.OnToolCall != nil {
					r.OnToolCall(tc.Function.Name, tc.Function.Arguments)
				}

				result, execErr := r.execute(tc.Function.Name, tc.Function.Arguments)
				if execErr != nil {
					result = fmt.Sprintf("error: %v", execErr)
				}

				if r.OnToolResult != nil {
					r.OnToolResult(tc.Function.Name, result)
				}

				messages = append(messages, ai.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
			}
			continue
		}

		// finish_reason == "stop" — the model produced its final answer.
		// Content was already streamed to `out` during the SSE phase.
		messages = append(messages, assistantMsg)
		return messages, nil
	}

	return messages, fmt.Errorf("agent exceeded maximum iterations (%d)", maxIterations)
}

// buildSchemas assembles the full list of ToolDefinitions the LLM sees:
// the agent's configured tools + memory tools + (optionally) delegate_to_agent.
func (r *Runner) buildSchemas(toolNames []string) []ai.ToolDefinition {
	schemas := tools.GetSchemas(toolNames)
	schemas = append(schemas, memorySetDef(), memoryGetDef(), memoryListDef())
	if r.depth < maxSubAgentDepth {
		schemas = append(schemas, delegateDef())
	}
	return schemas
}

// execute dispatches a tool call to the correct handler.
func (r *Runner) execute(name, argsJSON string) (string, error) {
	switch name {
	case "memory_set":
		return r.execMemorySet(argsJSON)
	case "memory_get":
		return r.execMemoryGet(argsJSON)
	case "memory_list":
		return r.execMemoryList()
	case "delegate_to_agent":
		return r.execDelegate(argsJSON)
	default:
		return tools.Execute(name, argsJSON)
	}
}

// ---- memory tools -------------------------------------------------------

func memorySetDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"key":   {"type": "string", "description": "Short identifier for this memory entry (e.g. 'user_name', 'task_goal')."},
			"value": {"type": "string", "description": "The information to remember."}
		},
		"required": ["key", "value"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "memory_set",
			Description: "Store a fact or note in session memory so it can be recalled in later turns. Overwrites any existing entry with the same key.",
			Parameters:  params,
		},
	}
}

func memoryGetDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"key": {"type": "string", "description": "Key of the memory entry to retrieve."}
		},
		"required": ["key"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "memory_get",
			Description: "Retrieve a specific stored fact from session memory by key. Call memory_list first if you are unsure which keys exist.",
			Parameters:  params,
		},
	}
}

func memoryListDef() ai.ToolDefinition {
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "memory_list",
			Description: "Return all key-value pairs currently stored in session memory.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
}

func (r *Runner) execMemorySet(argsJSON string) (string, error) {
	var args struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("memory_set: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Key) == "" {
		return "", fmt.Errorf("memory_set: key must not be empty")
	}
	r.Memory.Set(args.Key, args.Value)
	return fmt.Sprintf("stored %q", args.Key), nil
}

func (r *Runner) execMemoryGet(argsJSON string) (string, error) {
	var args struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("memory_get: invalid args: %w", err)
	}
	v, ok := r.Memory.Get(args.Key)
	if !ok {
		return fmt.Sprintf("(no entry for key %q)", args.Key), nil
	}
	return v, nil
}

func (r *Runner) execMemoryList() (string, error) {
	return r.Memory.String(), nil
}

// ---- sub-agent delegation -----------------------------------------------

func delegateDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"agent": {"type": "string", "description": "Name of the configured sub-agent to delegate to."},
			"task":  {"type": "string", "description": "A clear, self-contained description of what the sub-agent should accomplish. Include all context it needs — the sub-agent starts with an empty conversation."}
		},
		"required": ["agent", "task"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "delegate_to_agent",
			Description: "Delegate a self-contained sub-task to a specialised sub-agent. The sub-agent runs in an isolated context window and returns a condensed result. Use this when the task requires a specialisation outside your own role.",
			Parameters:  params,
		},
	}
}

func (r *Runner) execDelegate(argsJSON string) (string, error) {
	var args struct {
		Agent string `json:"agent"`
		Task  string `json:"task"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("delegate_to_agent: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Agent) == "" || strings.TrimSpace(args.Task) == "" {
		return "", fmt.Errorf("delegate_to_agent: agent and task are required")
	}

	// Open a fresh DB connection for the sub-agent lookup to avoid sharing
	// a connection across goroutines or nested calls.
	d, err := db.Connect()
	if err != nil {
		return "", fmt.Errorf("delegate_to_agent: cannot connect to database: %w", err)
	}
	defer d.Conn.Close()

	sub, err := GetAgent(d, args.Agent)
	if err != nil {
		return "", fmt.Errorf("delegate_to_agent: agent %q not found: %w", args.Agent, err)
	}

	subRunner := &Runner{
		ApiUrl: r.ApiUrl,
		ApiKey: r.ApiKey,
		Model:  r.Model,
		Memory: r.Memory, // share session memory with sub-agent
		depth:  r.depth + 1,
		// sub-agent callbacks are intentionally nil; the parent's OnToolCall
		// already signalled the delegation; noisy nested updates would confuse UI.
	}

	messages := []ai.Message{
		{Role: "system", Content: BuildSystemPrompt(sub.SystemPrompt)},
		{Role: "user", Content: args.Task},
	}

	var out strings.Builder
	if _, err := subRunner.Run(messages, sub.Tools, &out); err != nil {
		return "", fmt.Errorf("sub-agent %q: %w", args.Agent, err)
	}
	return out.String(), nil
}
