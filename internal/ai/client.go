package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ToolFunction describes the callable function within a tool definition.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolDefinition is the OpenAI tool object sent in chat requests.
type ToolDefinition struct {
	Type     string       `json:"type"` // always "function"
	Function ToolFunction `json:"function"`
}

// ToolCall is returned by the model when it wants to invoke a tool.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ChatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

type ChatResponseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type chatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
}

type ModelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func ListModels(apiUrl, apiKey string) ([]string, error) {
	// Derive models endpoint from chat/completions URL if possible
	modelsUrl := strings.Replace(apiUrl, "/chat/completions", "/models", 1)
	if modelsUrl == apiUrl {
		// If not found, try to assume it might need append if it's just a base URL
		if !strings.HasSuffix(modelsUrl, "/models") {
			modelsUrl = strings.TrimSuffix(modelsUrl, "/") + "/models"
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", modelsUrl, nil)
	if err != nil {
		return nil, err
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var list ModelList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range list.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

func StreamChat(apiUrl, apiKey, model, systemPrompt, userPrompt string, out io.Writer) error {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return StreamChatWithHistory(apiUrl, apiKey, model, messages, out)
}

func StreamChatWithHistory(apiUrl, apiKey, model string, messages []Message, out io.Writer) error {
	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" || line == "data: [DONE]" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := line[6:]
			var chunk ChatResponseChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				fmt.Fprint(out, content)
			}
		}
	}

	return nil
}

// chatOnce sends a non-streaming chat request and returns the assistant Message
// and finish_reason. Used by RunAgentWithHistory for the tool-calling loop.
func chatOnce(apiUrl, apiKey, model string, messages []Message, tools []ToolDefinition) (Message, string, error) {
	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Tools:    tools,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return Message{}, "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", apiUrl, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return Message{}, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Message{}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Message{}, "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return Message{}, "", err
	}
	if len(cr.Choices) == 0 {
		return Message{}, "", fmt.Errorf("empty response from API")
	}
	return cr.Choices[0].Message, cr.Choices[0].FinishReason, nil
}

// RunAgentWithHistory runs an agentic tool-calling loop. It calls the model,
// executes any requested tools via executor, and repeats until the model
// produces a final text answer or maxIterations is reached.
//
// onToolCall is notified (in the calling goroutine) before each tool
// execution so the caller can update UI state. executor receives the tool
// name and its raw JSON arguments and must return the result string.
//
// The final assistant text is written to out. The returned []Message slice
// contains the full updated history including tool-call turns.
func RunAgentWithHistory(
	apiUrl, apiKey, model string,
	messages []Message,
	tools []ToolDefinition,
	onToolCall func(name, args string),
	executor func(name, args string) (string, error),
	out io.Writer,
) ([]Message, error) {
	const maxIterations = 10
	for i := 0; i < maxIterations; i++ {
		assistantMsg, finishReason, err := chatOnce(apiUrl, apiKey, model, messages, tools)
		if err != nil {
			return messages, err
		}

		if finishReason == "tool_calls" || len(assistantMsg.ToolCalls) > 0 {
			messages = append(messages, assistantMsg)
			for _, tc := range assistantMsg.ToolCalls {
				if onToolCall != nil {
					onToolCall(tc.Function.Name, tc.Function.Arguments)
				}
				result, execErr := executor(tc.Function.Name, tc.Function.Arguments)
				if execErr != nil {
					result = fmt.Sprintf("error: %v", execErr)
				}
				messages = append(messages, Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
			}
			continue
		}

		// Final answer.
		messages = append(messages, assistantMsg)
		if _, err := io.WriteString(out, assistantMsg.Content); err != nil {
			return messages, err
		}
		return messages, nil
	}
	return messages, fmt.Errorf("agent exceeded maximum iterations (%d)", maxIterations)
}
