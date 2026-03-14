package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/mhai-org/mhai/internal/ai"
	"github.com/mhai-org/mhai/internal/config"
	"github.com/mhai-org/mhai/internal/persona"
	"github.com/mhai-org/mhai/internal/utils"
)

func LaunchInteractive(p *persona.Persona, provider *config.Provider) error {
	fmt.Printf("MHAI Interactive Mode - Persona: @%s\n", p.Name)
	fmt.Println("Type 'exit' or 'quit' to end session.")

	for {
		var userPrompt string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("You").
					Value(&userPrompt),
			),
		)

		err := form.Run()
		if err != nil {
			return err
		}

		userPrompt = strings.TrimSpace(userPrompt)
		if userPrompt == "" {
			continue
		}
		if userPrompt == "exit" || userPrompt == "quit" {
			break
		}

		fmt.Println("\nAI:")
		
		var fullResponse strings.Builder
		// Use a simple printer as out for now, later we can use glamour for chunks if possible
		// glamour works best on full blocks, but for streaming we'll just print raw and maybe re-render at the end
		err = ai.StreamChat(provider.ApiUrl, provider.ApiKey, "gpt-4", p.SystemPrompt, userPrompt, &utils.WriterWrapper{Builder: &fullResponse})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Println()

		// Re-render with glamour for final result
		out, _ := glamour.Render(fullResponse.String(), "dark")
		fmt.Print(out)
		fmt.Println()
	}

	return nil
}
