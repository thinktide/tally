package cli

import (
	"fmt"
	"os"

	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/sleep"
	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "tally",
	Short: "A CLI time tracking utility",
	Long:  `Tally is a command-line time tracking utility that helps you track time spent on projects.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB init for version command
		if cmd.Name() == "version" {
			return nil
		}

		if err := db.Init(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		// Check for sleep events during running timer
		if cmd.Name() != "config" && cmd.Name() != "status" {
			if err := sleep.CheckAndHandleSleep(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		db.Close()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(configCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("tally %s\n", Version)
	},
}
