package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
)

// statusCmd provides functionality to display the current timer status.
//
// The command retrieves any actively running or paused timer from the database and shows relevant details like project, title, tags,
// start time, elapsed duration, and pause information.
//
// If there are no active timers, the command informs the user accordingly. It is primarily executed via the [RunE] handler [runStatus].
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current timer status",
	RunE:  runStatus,
}

// runStatus retrieves the currently running or paused timer entry from the database and prints its status.
//
// If no timer is currently running, the message "No timer running" is printed. Otherwise, detailed information
// about the running or paused timer, including its duration, associated project, title, tags, and pause details,
// is displayed.
//
// cmd:
//   - The [cobra.Command] context in which this function is called.
//
// args:
//   - A slice of strings representing command-line arguments passed to the function.
//
// Returns an [error] if the retrieval of the running entry from the database fails or any other runtime issue occurs.
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

// printStatus formats and prints the details of a time entry to the console.
//
// The function displays the status (e.g., "Running" or "Paused") along with the project name and, if present, the title and tags.
// It also shows the start time, elapsed duration, and total pause time with the number of pauses, if applicable.
//
// entry:
//   - A pointer to [model.Entry] containing details of the time entry such as start time, status, title, tags, and pauses.
//
// The output is formatted into a readable structure for display in a CLI environment.
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

// formatDuration formats a [time.Duration] into a human-readable string with hours, minutes, and seconds.
//
// The function rounds the duration to the nearest second and returns a string representation:
//   - If the duration is at least an hour, the format will include hours, minutes, and seconds (e.g., "2h 15m 30s").
//   - If the duration is less than an hour but at least a minute, the format will include minutes and seconds (e.g., "45m 30s").
//   - For durations less than a minute, only seconds are included (e.g., "30s").
//
// Returns the formatted duration string.
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

// formatDurationShort formats a [time.Duration] into a short and concise string representation rounded to minutes.
//
// Durations less than one hour are formatted as "Xm". Durations of one hour or more are formatted as "Xh Xm".
//
// d is the [time.Duration] value to format.
//
// Returns the formatted string representing the duration.
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
