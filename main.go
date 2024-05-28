package main

import (
	"GADS/hub"
	"GADS/provider"
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var AppVersion = "development"

func main() {
	var rootCmd = &cobra.Command{Use: "GADS"}
	rootCmd.PersistentFlags().String("host-address", "localhost", "The IP address of the host machine")
	rootCmd.PersistentFlags().String("port", "", "The port on which the component should run")
	rootCmd.PersistentFlags().String("mongo-db", "localhost:27017", "The address of the MongoDB instance")

	// Hub Command
	var hubCmd = &cobra.Command{
		Use:   "hub",
		Short: "Run a hub component",
		Run: func(cmd *cobra.Command, args []string) {
			hub.StartHub(cmd.Flags())
		},
	}
	hubCmd.Flags().String("ui-files-dir", "", "Directory where the UI static files will be unpacked and served from."+
		"\nBy default app will try to use a temp dir on the host, use this flag only if you encounter issues with the temp folder."+
		"\nAlso you need to have created the folder in advance!")
	rootCmd.AddCommand(hubCmd)

	// Provider Command
	var providerCmd = &cobra.Command{
		Use:   "provider",
		Short: "Run a provider component",
		Run: func(cmd *cobra.Command, args []string) {
			provider.StartProvider(cmd.Flags())
		},
	}
	providerCmd.Flags().String("nickname", "", "Nickname of the provider")
	providerCmd.Flags().String("provider-folder", ".", "The folder where logs and other data will be stored")
	providerCmd.Flags().String("log-level", "info", "The verbosity of the logs of the provider instance")
	rootCmd.AddCommand(providerCmd)

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the application version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(AppVersion)
		},
	}
	rootCmd.AddCommand(versionCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
