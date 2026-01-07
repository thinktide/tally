package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
)

// Version indicates the current build version of the application. Defaults to "dev" if not explicitly set.
var Version = "dev"

// rootCmd is the primary command for the CLI, serving as the entry point for all subcommands.
//
// It initializes necessary resources like the database before executing a command.
// On completion, it ensures resources such as the database connection are properly closed.
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

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		db.Close()
	},
}

// Execute runs the root command of the CLI application.
//
// It initializes the command execution flow by invoking [rootCmd.Execute].
// If an error occurs during execution, the function terminates the program with a non-zero exit code.
//
// This function handles all CLI commands, ensuring necessary pre-run and post-run tasks defined in [rootCmd] are executed.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// init initializes the root command by adding all subcommands to it.
//
// This function is executed automatically when the package is imported. It registers subcommands such as [versionCmd],
// [startCmd], [stopCmd], [statusCmd], [pauseCmd], [resumeCmd], [logCmd], [editCmd], [deleteCmd], [reportCmd], and [configCmd],
// enabling the CLI's functionality. It ensures all commands are integrated with the application's root command.
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

// versionCmd represents the command to print the application's version number.
//
// When executed, this command outputs the current version of the application. The version is stored in the [Version] variable.
//
// This command does not require initialization of other subsystems like the database, ensuring quick response time. It is useful for verifying
// the installed version or debugging issues related to versioning.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("tally %s\n", Version)
	},
}
