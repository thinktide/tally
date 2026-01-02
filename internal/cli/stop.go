package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
)

// stopCmd is a CLI command used to stop the currently running time entry.
//
// The command locates the running time entry in the database and sets its end time, marking it as stopped.
// If no timer is running, it provides a user-friendly message indicating this.
// After stopping the timer, it calculates the total duration of the time entry, excluding any pauses, and displays it.
//
// Errors are returned if the database fails to fetch the running entry or to update the entry's status. The output
// also includes relevant details like the associated project and title when available.
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current time entry",
	RunE:  runStop,
}

// runStop stops the currently running time entry.
//
// If there is no running timer, the function prints a message indicating this and exits without error.
//
// The function interacts with the database to stop the running entry and reloads it to retrieve updated details.
// It calculates and formats the time duration between the start and stop of the entry.
//
// Errors:
//   - Returns an error if fetching the running entry fails.
//   - Returns an error if stopping the entry in the database fails.
//   - Returns an error if the reloaded entry cannot be fetched.
//
// Prints a message summarizing the stopped timer, including the project name, optional title, and duration.
func runStop(cmd *cobra.Command, args []string) error {
	entry, err := db.GetRunningEntry()
	if err != nil {
		return fmt.Errorf("failed to get running entry: %w", err)
	}
	if entry == nil {
		fmt.Println("No timer running")
		return nil
	}

	if err := db.StopEntry(entry.ID); err != nil {
		return fmt.Errorf("failed to stop entry: %w", err)
	}

	// Reload entry to get updated data
	entry, err = db.GetEntryByID(entry.ID)
	if err != nil {
		return fmt.Errorf("failed to reload entry: %w", err)
	}

	duration := entry.Duration()
	fmt.Printf("Stopped timer for @%s", entry.Project.Name)
	if entry.Title != "" {
		fmt.Printf(": %s", entry.Title)
	}
	fmt.Printf(" [%s]\n", formatDuration(duration))

	return nil
}
