package cmd

import (
	"fmt"
	"os"
	"strings"

	"time"

	"github.com/charmbracelet/glamour"
	"github.com/mhai-org/term-ai/internal/agent"
	"github.com/mhai-org/term-ai/internal/ai"
	"github.com/mhai-org/term-ai/internal/config"
	"github.com/mhai-org/term-ai/internal/db"
	"github.com/mhai-org/term-ai/internal/memory"
	"github.com/mhai-org/term-ai/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var promptFlag string

// Version is set at build time via -ldflags.
var Version = "dev"

func warnIfDevVersion() {
	if Version == "dev" {
		fmt.Fprintf(os.Stderr, "Warning: Running with default version 'dev'. This build may not correspond to a tagged release.\n")
	}
}

var rootCmd = &cobra.Command{
	Use:     "ai [persona]",
	Version: Version,
	Short:   "term-ai - minimalist Golang-based CLI tool for AI interaction",
	Long: `term-ai is a beautiful, interactive CLI tool for interacting with multi-modal AI personas.
It supports both an interactive TUI mode and a direct output mode.`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		warnIfDevVersion()
		personaName := ""
		
		// If prompt flag is empty, try to gather from positional args
		remainingArgs := args
		if len(args) > 0 {
			if strings.HasPrefix(args[0], "@") {
				personaName = strings.TrimPrefix(args[0], "@")
				remainingArgs = args[1:]
			}
		}

		if promptFlag == "" && len(remainingArgs) > 0 {
			promptFlag = strings.Join(remainingArgs, " ")
		}

		d, err := db.Connect()
		if err != nil {
			fmt.Printf("Error connecting to database: %v\n", err)
			return
		}
		defer d.Conn.Close()

		if err := d.InitSchema(); err != nil {
			fmt.Printf("Error initializing schema: %v\n", err)
			return
		}

		// For TUI mode with no explicit @agent arg, fall back to saved default.
		if personaName == "" {
			if saved, err := config.GetConfig(d, "default_tui_agent"); err == nil && saved != "" {
				personaName = saved
			} else {
				personaName = "default"
			}
		}

		p, err := agent.GetAgent(d, personaName)
		if err != nil {
			if personaName == "default" {
				p = &agent.Agent{Name: "default", SystemPrompt: "You are a helpful assistant."}
			} else {
				fmt.Printf("Agent '%s' not found.\n", personaName)
				return
			}
		}

		// Load stored config or default
		providerName, _ := config.GetConfig(d, "active_provider")
		modelName, _ := config.GetConfig(d, "active_model")
		if modelName == "" {
			modelName = "gpt-4"
		}

		var provider *config.Provider
		if providerName != "" {
			p, err := config.GetProvider(d, providerName)
			if err == nil {
				provider = p
			}
		}

		if provider == nil {
			providers, err := config.ListProviders(d)
			if err != nil || len(providers) == 0 {
				fmt.Println("No providers configured. Please use 'ai config set-provider' first.")
				return
			}
			provider = &providers[0]
			// Persist this as the default if it's the first time
			config.SetConfig(d, "active_provider", provider.Name)
		}

		if promptFlag != "" {
			// Direct Mode
			var fullResponse strings.Builder
			isTerminal := term.IsTerminal(int(os.Stdout.Fd()))

			// If the agent has tools configured, use the agentic ReAct loop
			// instead of plain streaming. Memory is ephemeral per invocation.
			if len(p.Tools) > 0 {
				messages := []ai.Message{
					{Role: "system", Content: agent.BuildSystemPrompt(p.SystemPrompt)},
					{Role: "user", Content: promptFlag},
				}
				r := &agent.Runner{
					ApiUrl: provider.ApiUrl,
					ApiKey: provider.ApiKey,
					Model:  modelName,
					Memory: memory.New(),
					OnToolCall: func(name, args string) {
						fmt.Fprintf(os.Stderr, "🔧 %s %s\n", name, args)
					},
				}
				if _, err := r.Run(messages, p.Tools, &fullResponse); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return
				}
				if isTerminal {
					out, _ := glamour.Render(fullResponse.String(), "dark")
					fmt.Print(out)
				} else {
					fmt.Print(fullResponse.String())
				}
				return
			}

			if isTerminal {
				// Resolve the conversation state before starting the UI program,
				// so we can pass the resume message as initial state (avoids deadlock
				// from calling prog.Send before prog.Run).
				var lastConv *ai.Conversation
				resumeMsg := ""
				{
					dbConn, _ := db.Connect()
					if dbConn != nil {
						lastConv, _ = ai.GetRecentConversation(dbConn, "cli")
						dbConn.Conn.Close()
					}
				}

				messages := []ai.Message{{Role: "system", Content: agent.BuildSystemPrompt(p.SystemPrompt)}}
				if lastConv != nil && time.Since(lastConv.UpdatedAt) < 5*time.Minute {
					messages = lastConv.History
					resumeMsg = fmt.Sprintf("Resuming recent conversation from %s...", lastConv.UpdatedAt.Format("15:04"))
				} else {
					lastConv = nil // Start fresh
				}
				messages = append(messages, ai.Message{Role: "user", Content: promptFlag})

				ctxTokens := 0
				for _, msg := range messages {
					ctxTokens += len(msg.Content)
				}
				ctxTokens /= 4

				prog, tokenChan, doneChan := ui.RunStatusProgram(resumeMsg, ctxTokens)

				go func() {
					if _, err := prog.Run(); err != nil {
						fmt.Fprintf(os.Stderr, "Error UI: %v\n", err)
					}
				}()

				writer := &ui.TokenCounterWriter{
					Writer:    &fullResponse,
					TokenChan: tokenChan,
				}

				err := ai.StreamChatWithHistory(provider.ApiUrl, provider.ApiKey, modelName, messages, writer)
				doneChan <- true
				time.Sleep(50 * time.Millisecond)

				if err != nil {
					fmt.Printf("\nError: %v\n", err)
				} else {
					// Save/Update
					d, _ := db.Connect()
					if d != nil {
						messages = append(messages, ai.Message{Role: "assistant", Content: fullResponse.String()})
						if lastConv != nil {
							lastConv.History = messages
							ai.SaveConversation(d, lastConv)
						} else {
							title := promptFlag
							if len(title) > 30 {
								title = title[:27] + "..."
							}
							ai.SaveConversation(d, &ai.Conversation{
								Title:        title,
								History:      messages,
								Platform:     "cli",
								ProviderName: provider.Name,
								ModelName:    modelName,
								PersonaName:  p.Name,
							})
						}
						d.Conn.Close()
					}
				}

				out, _ := glamour.Render(fullResponse.String(), "dark")
				fmt.Print(out)
			} else {
				// Non-terminal: just one-off as before
				err := ai.StreamChat(provider.ApiUrl, provider.ApiKey, modelName, agent.BuildSystemPrompt(p.SystemPrompt), promptFlag, &fullResponse)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
				fmt.Print(fullResponse.String())
			}
		} else {
			// Interactive Mode
			if err := ui.LaunchInteractive(p, provider, modelName); err != nil {
				fmt.Printf("Interactive session error: %v\n", err)
			}
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&promptFlag, "prompt", "p", "", "Your prompt for the AI")
}
