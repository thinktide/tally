package cli

import (
	"fmt"

	"github.com/jdecarlo/tally/internal/db"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current time entry",
	RunE:  runStop,
}

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
