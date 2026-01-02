package cli

import (
	"fmt"
	"time"

	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current timer status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	entry, err := db.GetRunningEntry()
	if err != nil {
		return fmt.Errorf("failed to get running entry: %w", err)
	}
	if entry == nil {
		fmt.Println("No timer running")
		return nil
	}

	printStatus(entry)
	return nil
}

func printStatus(entry *model.Entry) {
	duration := entry.Duration()
	status := "Running"
	if entry.Status == model.StatusPaused {
		status = "Paused"
	}

	fmt.Printf("[%s] @%s", status, entry.Project.Name)
	if entry.Title != "" {
		fmt.Printf(": %s", entry.Title)
	}
	if len(entry.Tags) > 0 {
		fmt.Printf(" [%s]", formatTagsFromModel(entry.Tags))
	}
	fmt.Printf("\n")
	fmt.Printf("  Started: %s\n", entry.StartTime.Format("15:04:05"))
	fmt.Printf("  Elapsed: %s\n", formatDuration(duration))

	if len(entry.Pauses) > 0 {
		var totalPause time.Duration
		for _, p := range entry.Pauses {
			totalPause += p.Duration()
		}
		fmt.Printf("  Paused:  %s (%d pause(s))\n", formatDuration(totalPause), len(entry.Pauses))
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatDurationShort(d time.Duration) string {
	d = d.Round(time.Minute)

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
