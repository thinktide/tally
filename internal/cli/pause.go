package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
)

var (
	pauseFrom string
	pauseTo   string
)

// pauseCmd represents a command to pause the currently running timer.
//
// This command updates the status of a running timer to "paused" and records the pause event in the database.
//
// If no timer is running, the command informs the user. If the timer is already paused, it notifies the user of its current state.
var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the current timer",
	Long: `Pause the current timer or record a historical pause.

Examples:
  tally pause                    # Pause now
  tally pause -f 09:00           # Record pause from 9am to now
  tally pause -f 09:00 -t 10:30  # Record pause from 9am to 10:30am`,
	RunE: runPause,
}

func init() {
	pauseCmd.Flags().StringVarP(&pauseFrom, "from", "f", "", "Pause start time (HH:MM or YYYY-MM-DD HH:MM:SS)")
	pauseCmd.Flags().StringVarP(&pauseTo, "to", "t", "", "Pause end time (HH:MM or YYYY-MM-DD HH:MM:SS)")
}

// parseTimeInput parses a time string in various formats.
// Supports: "HH:MM", "HH:MM:SS", "YYYY-MM-DD HH:MM:SS"
func parseTimeInput(input string) (time.Time, error) {
	now := time.Now()

	// Try full datetime first
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", input, time.Local); err == nil {
		return t, nil
	}

	// Try HH:MM:SS (today)
	if t, err := time.ParseInLocation("15:04:05", input, time.Local); err == nil {
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
	}

	// Try HH:MM (today)
	if t, err := time.ParseInLocation("15:04", input, time.Local); err == nil {
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s (use HH:MM, HH:MM:SS, or YYYY-MM-DD HH:MM:SS)", input)
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

	// Handle historical pause with --from flag
	if pauseFrom != "" {
		fromTime, err := parseTimeInput(pauseFrom)
		if err != nil {
			return err
		}

		// Validate from time is after entry start
		if fromTime.Before(entry.StartTime) {
			return fmt.Errorf("pause start time cannot be before entry start time (%s)", entry.StartTime.Format("15:04:05"))
		}

		// Default to now if --to not specified
		toTime := time.Now()
		if pauseTo != "" {
			toTime, err = parseTimeInput(pauseTo)
			if err != nil {
				return err
			}
		}

		// Validate to time is after from time
		if toTime.Before(fromTime) {
			return fmt.Errorf("pause end time cannot be before pause start time")
		}

		// Create the historical pause (completed, doesn't change entry status)
		_, err = db.CreatePause(entry.ID, fromTime, &toTime, "Manual")
		if err != nil {
			return fmt.Errorf("failed to create pause: %w", err)
		}

		fmt.Printf("Added pause: %s - %s (%s)\n",
			fromTime.Format("15:04:05"),
			toTime.Format("15:04:05"),
			formatDuration(toTime.Sub(fromTime)))
		return nil
	}

	// Regular pause (pause now)
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
