package cmd

import (
	"fmt"

	"github.com/mhai-org/term-ai/internal/db"
	"github.com/mhai-org/term-ai/internal/persona"
	"github.com/spf13/cobra"
)

var personaCmd = &cobra.Command{
	Use:   "persona",
	Short: "Manage AI personas",
}

var personaSetCmd = &cobra.Command{
	Use:   "set [name] [prompt]",
	Short: "Create or update a persona",
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

		if err := persona.SetPersona(d, name, prompt); err != nil {
			fmt.Printf("Error saving persona: %v\n", err)
			return
		}
		fmt.Printf("Persona '%s' saved.\n", name)
	},
}

var personaUnsetCmd = &cobra.Command{
	Use:   "unset [name]",
	Short: "Delete a persona",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		
		d, err := db.Connect()
		if err != nil {
			fmt.Printf("Error connecting to database: %v\n", err)
			return
		}
		defer d.Conn.Close()

		if err := persona.UnsetPersona(d, name); err != nil {
			fmt.Printf("Error deleting persona: %v\n", err)
			return
		}
		fmt.Printf("Persona '%s' deleted.\n", name)
	},
}

var personaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured personas",
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

		personas, err := persona.ListPersonas(d)
		if err != nil {
			fmt.Printf("Error listing personas: %v\n", err)
			return
		}

		if len(personas) == 0 {
			fmt.Println("No personas configured.")
			return
		}

		fmt.Println("Configured personas:")
		for _, p := range personas {
			fmt.Printf("- %s\n", p.Name)
		}
	},
}

func init() {
	rootCmd.AddCommand(personaCmd)
	personaCmd.AddCommand(personaSetCmd)
	personaCmd.AddCommand(personaUnsetCmd)
	personaCmd.AddCommand(personaListCmd)
}
