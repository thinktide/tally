package cli

import (
	"fmt"

	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the current timer",
	RunE:  runPause,
}

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

	if err := db.PauseEntry(entry.ID); err != nil {
		return fmt.Errorf("failed to pause entry: %w", err)
	}

	fmt.Printf("Paused timer for @%s", entry.Project.Name)
	if entry.Title != "" {
		fmt.Printf(": %s", entry.Title)
	}
	fmt.Printf(" [%s elapsed]\n", formatDuration(entry.Duration()))

	return nil
}
