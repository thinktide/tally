package cli

import (
	"fmt"

	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the paused timer",
	RunE:  runResume,
}

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
