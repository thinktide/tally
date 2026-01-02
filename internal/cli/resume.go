package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
)

// resumeCmd represents a command to resume a paused timer.
//
// This command updates the status of a paused timer back to "running" and continues tracking time.
//
// If no timer is paused, the command informs the user. If the timer is already running, it displays the current status.
var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the paused timer",
	RunE:  runResume,
}

// runResume resumes a previously paused timer for an active project.
//
// If there is no active or paused timer, it prints a message indicating so and exits successfully.
// If the timer is already running, it prints the current status without modification.
//
// The function retrieves the currently running or paused entry from the database using [db.GetRunningEntry].
// If the entry is paused, it updates its status to running using [db.ResumeEntry] and prints a confirmation message.
// If resumption fails due to a database error, it returns the error.
//
// - cmd: The Cobra command instance triggering this function.
// - args: Additional arguments passed with the command.
//
// Returns an error if the database operations fail or if the timer resumption process encounters issues.
func runResume(cmd *cobra.Command, args []string) error {
	entry, err := db.GetRunningEntry()
	if err != nil {
		return fmt.Errorf("failed to get running entry: %w", err)
	}
	if entry == nil {
		fmt.Println("No timer to resume")
		return nil
	}

	if entry.Status == model.StatusRunning {
		fmt.Println("Timer is already running")
		printStatus(entry)
		return nil
	}

	if err := db.ResumeEntry(entry.ID); err != nil {
		return fmt.Errorf("failed to resume entry: %w", err)
	}

	fmt.Printf("Resumed timer for @%s", entry.Project.Name)
	if entry.Title != "" {
		fmt.Printf(": %s", entry.Title)
	}
	fmt.Println()

	return nil
}
