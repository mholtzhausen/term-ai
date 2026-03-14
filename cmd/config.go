package cmd

import (
	"fmt"

	"github.com/mhai-org/mhai/internal/config"
	"github.com/mhai-org/mhai/internal/db"
	"github.com/spf13/cobra"
)

var (
	providerKey string
	providerUrl string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration and providers",
}

var setProviderCmd = &cobra.Command{
	Use:   "set-provider [name]",
	Short: "Configure a provider",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		
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

		if err := config.SetProvider(d, name, providerKey, providerUrl); err != nil {
			fmt.Printf("Error saving provider: %v\n", err)
			return
		}
		fmt.Printf("Provider '%s' configured.\n", name)
	},
}

var listProvidersCmd = &cobra.Command{
	Use:   "list-providers",
	Short: "List configured providers",
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

		providers, err := config.ListProviders(d)
		if err != nil {
			fmt.Printf("Error listing providers: %v\n", err)
			return
		}

		if len(providers) == 0 {
			fmt.Println("No providers configured.")
			return
		}

		fmt.Println("Configured providers:")
		for _, p := range providers {
			fmt.Printf("- %s (URL: %s)\n", p.Name, p.ApiUrl)
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setProviderCmd)
	configCmd.AddCommand(listProvidersCmd)

	setProviderCmd.Flags().StringVar(&providerKey, "key", "", "API_KEY for the provider")
	setProviderCmd.Flags().StringVar(&providerUrl, "url", "", "ENDPOINT_URL for the provider")
	setProviderCmd.MarkFlagRequired("key")
}
