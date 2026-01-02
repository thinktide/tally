package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

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
	Short: "Resume a paused or stopped timer",
	Long: `Resume a paused timer or reopen a stopped entry.

If a timer is paused, it resumes immediately.
If the last entry is stopped, it shows details and asks for confirmation.
Reopening a stopped entry creates a pause from the stop time to now.`,
	RunE: runResume,
}

func runResume(cmd *cobra.Command, args []string) error {
	// First check for running/paused entry
	entry, err := db.GetRunningEntry()
	if err != nil {
		return fmt.Errorf("failed to get running entry: %w", err)
	}

	// If we have a running/paused entry, handle it
	if entry != nil {
		if entry.Status == model.StatusRunning {
			fmt.Println("Timer is already running")
			printStatus(entry)
			return nil
		}

		// Resume paused entry
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

	// No running/paused entry - check for last stopped entry
	lastEntry, err := db.GetLastEntry()
	if err != nil {
		return fmt.Errorf("failed to get last entry: %w", err)
	}
	if lastEntry == nil {
		fmt.Println("No timer to resume")
		return nil
	}

	if lastEntry.Status != model.StatusStopped {
		fmt.Println("No timer to resume")
		return nil
	}

	// Show entry details and ask for confirmation
	fmt.Println("Last entry:")
	fmt.Printf("  Project: @%s\n", lastEntry.Project.Name)
	if lastEntry.Title != "" {
		fmt.Printf("  Title:   %s\n", lastEntry.Title)
	}
	if len(lastEntry.Tags) > 0 {
		fmt.Printf("  Tags:    %s\n", formatTagsFromModel(lastEntry.Tags))
	}
	fmt.Printf("  Started: %s\n", lastEntry.StartTime.Format("2006-01-02 15:04:05"))
	if lastEntry.EndTime != nil {
		fmt.Printf("  Stopped: %s\n", lastEntry.EndTime.Format("2006-01-02 15:04:05"))
		gap := time.Since(*lastEntry.EndTime)
		fmt.Printf("  Gap:     %s ago\n", formatDuration(gap))
	}
	fmt.Println()

	fmt.Print("Reopen this entry? A pause will be created for the gap. [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	input = strings.TrimSpace(strings.ToLower(input))

	if input != "y" && input != "yes" {
		fmt.Println("Cancelled")
		return nil
	}

	// Create pause for the gap (from end_time to now)
	if lastEntry.EndTime != nil {
		now := time.Now()
		_, err = db.CreatePause(lastEntry.ID, *lastEntry.EndTime, &now, "Manual")
		if err != nil {
			return fmt.Errorf("failed to create pause: %w", err)
		}
	}

	// Reopen the entry
	if err := db.ReopenEntry(lastEntry.ID); err != nil {
		return fmt.Errorf("failed to reopen entry: %w", err)
	}

	fmt.Printf("Reopened timer for @%s", lastEntry.Project.Name)
	if lastEntry.Title != "" {
		fmt.Printf(": %s", lastEntry.Title)
	}
	fmt.Println()

	return nil
}
