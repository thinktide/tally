package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
)

// pauseCmd represents a command to pause the currently running timer.
//
// This command updates the status of a running timer to "paused" and records the pause event in the database.
//
// If no timer is running, the command informs the user. If the timer is already paused, it notifies the user of its current state.
var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the current timer",
	RunE:  runPause,
}

// runPause halts the currently running timer, changing its status to paused.
//
// The function first identifies the active timer entry using [db.GetRunningEntry]. If no such entry exists,
// it prints a message indicating that no timer is running and exits successfully.
//
// If the detected timer entry is already paused (status [model.StatusPaused]), it notifies the user and displays
// the current status using [printStatus].
//
// Otherwise, it executes the pause operation via [db.PauseEntry], associates it with the reason "Manual," and
// updates the timer's status to paused.
//
// The function then confirms the pause by printing appropriate feedback, including the associated project name,
// an optional title, and the timer's elapsed duration.
//
// Returns an error in cases like failing to fetch the active entry or issues during the database update.
func runPause(cmd *cobra.Command, args []string) error {
	entry, err := db.GetRunningEntry()
	if err != nil {
		return fmt.Errorf("failed to get running entry: %w", err)
	}
	if entry == nil {
		fmt.Println("No timer running")
		return nil
	}

	if entry.Status == model.StatusPaused {
		fmt.Println("Timer is already paused")
		printStatus(entry)
		return nil
	}

	if err := db.PauseEntry(entry.ID, "Manual"); err != nil {
		return fmt.Errorf("failed to pause entry: %w", err)
	}

	fmt.Printf("Paused timer for @%s", entry.Project.Name)
	if entry.Title != "" {
		fmt.Printf(": %s", entry.Title)
	}
	fmt.Printf(" [%s elapsed]\n", formatDuration(entry.Duration()))

	return nil
}
