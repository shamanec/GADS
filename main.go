package main

import (
	"GADS/hub"
	"GADS/provider"
	"embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var AppVersion = "development"

//go:embed resources
var resourceFiles embed.FS

func main() {
	var rootCmd = &cobra.Command{Use: "GADS"}
	rootCmd.PersistentFlags().String("mongo-db", "localhost:27017", "The address of the MongoDB instance")

	// Hub Command
	var hubCmd = &cobra.Command{
		Use:   "hub",
		Short: "Run a hub component",
		Run: func(cmd *cobra.Command, args []string) {
			hub.StartHub(cmd.Flags(), AppVersion, uiFiles, resourceFiles)
		},
	}
	hubCmd.Flags().String("host-address", "localhost", "The IP address of the host machine")
	hubCmd.Flags().String("port", "", "The port on which the component should run")
	hubCmd.Flags().Bool("auth", true, "Enable or disable authentication on hub endpoints, default is `true`")
	hubCmd.Flags().String("files-dir", "", "Directory where resource files will be unpacked."+
		"\nBy default app will try to use a temp dir on the host, use this flag only if you encounter issues with the temp folder."+
		"\nAlso you need to have created the folder in advance!")
	rootCmd.AddCommand(hubCmd)

	// Provider Command
	var providerCmd = &cobra.Command{
		Use:   "provider",
		Short: "Run a provider component",
		Run: func(cmd *cobra.Command, args []string) {
			provider.StartProvider(cmd.Flags(), resourceFiles)
		},
	}
	providerCmd.Flags().String("nickname", "", "Nickname of the provider")
	providerCmd.Flags().String("provider-folder", ".", "The folder where logs and other data will be stored")
	providerCmd.Flags().String("log-level", "info", "The verbosity of the logs of the provider instance")
	providerCmd.Flags().String("hub", "", "The address of the GADS hub instance")
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
