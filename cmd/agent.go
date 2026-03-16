package cmd

import (
	"fmt"

	"github.com/mhai-org/term-ai/internal/agent"
	"github.com/mhai-org/term-ai/internal/db"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage AI agents",
}

var agentTools []string

var agentSetCmd = &cobra.Command{
	Use:   "set [name] [prompt]",
	Short: "Create or update an agent",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		prompt := args[1]

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

		if err := agent.SetAgent(d, name, prompt, agentTools); err != nil {
			fmt.Printf("Error saving agent: %v\n", err)
			return
		}
		fmt.Printf("Agent '%s' saved.\n", name)
	},
}

var agentUnsetCmd = &cobra.Command{
	Use:   "unset [name]",
	Short: "Delete an agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		d, err := db.Connect()
		if err != nil {
			fmt.Printf("Error connecting to database: %v\n", err)
			return
		}
		defer d.Conn.Close()

		if err := agent.UnsetAgent(d, name); err != nil {
			fmt.Printf("Error deleting agent: %v\n", err)
			return
		}
		fmt.Printf("Agent '%s' deleted.\n", name)
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured agents",
	Run: func(cmd *cobra.Command, args []string) {
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

		agents, err := agent.ListAgents(d)
		if err != nil {
			fmt.Printf("Error listing agents: %v\n", err)
			return
		}

		if len(agents) == 0 {
			fmt.Println("No agents configured.")
			return
		}

		fmt.Println("Configured agents:")
		for _, a := range agents {
			fmt.Printf("- %s\n", a.Name)
		}
	},
}

func init() {
	agentSetCmd.Flags().StringSliceVar(&agentTools, "tools", nil, "Comma-separated list of tool names (e.g. bash,read_file,write_file)")
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentSetCmd)
	agentCmd.AddCommand(agentUnsetCmd)
	agentCmd.AddCommand(agentListCmd)
}
