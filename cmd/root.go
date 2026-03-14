package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mhai-org/mhai/internal/ai"
	"github.com/mhai-org/mhai/internal/config"
	"github.com/mhai-org/mhai/internal/db"
	"github.com/mhai-org/mhai/internal/persona"
	"github.com/mhai-org/mhai/internal/ui"
	"github.com/mhai-org/mhai/internal/utils"
	"github.com/spf13/cobra"
	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

var promptFlag string

var rootCmd = &cobra.Command{
	Use:   "ai [persona]",
	Short: "MHAI - minimalist Golang-based CLI tool for AI interaction",
	Long: `MHAI is a beautiful, interactive CLI tool for interacting with multi-modal AI personas.
It supports both an interactive TUI mode and a direct output mode.`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		personaName := "default"
		
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

		p, err := persona.GetPersona(d, personaName)
		if err != nil {
			if personaName == "default" {
				p = &persona.Persona{Name: "default", SystemPrompt: "You are a helpful assistant."}
			} else {
				fmt.Printf("Persona '%s' not found.\n", personaName)
				return
			}
		}

		// Use the first available provider as default
		providers, err := config.ListProviders(d)
		if err != nil || len(providers) == 0 {
			fmt.Println("No providers configured. Please use 'ai config set-provider' first.")
			return
		}
		provider := providers[0]

		if promptFlag != "" {
			// Direct Mode
			var fullResponse strings.Builder
			
			// If it's a terminal, we might want to show streaming raw, then re-render bits
			// Or just stream raw and leave it if we want to keep it simple.
			// But PRD says "formatted with glamour".
			
			isTerminal := term.IsTerminal(int(os.Stdout.Fd()))

			err := ai.StreamChat(provider.ApiUrl, provider.ApiKey, "gpt-4", p.SystemPrompt, promptFlag, &utils.WriterWrapper{Builder: &fullResponse, Silent: !isTerminal})
			if err != nil {
				fmt.Printf("\nError: %v\n", err)
			}
			
			if isTerminal {
				fmt.Println("\n---")
				out, _ := glamour.Render(fullResponse.String(), "dark")
				fmt.Print(out)
			} else {
				fmt.Print(fullResponse.String())
			}
		} else {
			// Interactive Mode
			if err := ui.LaunchInteractive(p, &provider); err != nil {
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
