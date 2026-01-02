package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
)

// logLimit defines the maximum number of log entries to process in a single operation.
//
// logFrom specifies the starting point or source of the logs.
//
// logTo specifies the endpoint or destination for the logs.
var (
	logLimit int
	logFrom  string
	logTo    string
)

// logCmd represents a CLI command to display time entries, optionally filtered by project, tags, or date ranges.
//
// Use filters like `@project` for project-specific entries or `+tag` for entries with specific tags.
//
// The command supports flags for limiting the number of entries shown (--limit) and setting date ranges (--from and --to).
var logCmd = &cobra.Command{
	Use:   "log [@project] [+tag]...",
	Short: "Show time entries",
	Long: `Show time entries, optionally filtered by project and tags.

Examples:
  tally log                    # Last 10 entries
  tally log --limit 20         # Last 20 entries
  tally log @work              # Entries for 'work' project
  tally log +backend           # Entries with 'backend' tag
  tally log @work +backend     # Entries for 'work' with 'backend' tag`,
	RunE: runLog,
}

// init initializes flags for the [logCmd] command.
//
// It sets up the following flags:
//   - "limit" (-n): An integer flag specifying the number of log entries to show (default: 10).
//   - "from": A string flag specifying the start date in YYYY-MM-DD format.
//   - "to": A string flag specifying the end date in YYYY-MM-DD format.
func init() {
	logCmd.Flags().IntVarP(&logLimit, "limit", "n", 10, "Number of entries to show")
	logCmd.Flags().StringVar(&logFrom, "from", "", "Start date (YYYY-MM-DD)")
	logCmd.Flags().StringVar(&logTo, "to", "", "End date (YYYY-MM-DD)")
}

// runLog executes the logic to retrieve and display time entries based on given filters.
//
// This function supports filtering entries by project (prefixed with "@") or tags (prefixed with "+").
// It also allows date filtering using global options `logFrom` and `logTo`. The results are
// capped by a global `logLimit` value.
//
// Entries are retrieved through [db.ListEntries], and when a project or tag is specified,
// they are resolved via [db.GetProjectByName] or [db.GetTagByName]. For invalid project or tag names,
// the function outputs a message and exits gracefully without errors.
//
// Parameters:
//   - cmd: The [*cobra.Command] triggering this execution.
//   - args: Additional CLI arguments specifying filters (e.g., project or tags).
//
// Returns:
//   - An error if database operations fail or if invalid date formats are detected.
//   - Otherwise, a list of matching entries is printed to the console, and `nil` is returned.
func runLog(cmd *cobra.Command, args []string) error {
	opts := db.ListEntriesOptions{
		Limit: logLimit,
	}

	// Parse filters from args
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			projectName := strings.TrimPrefix(arg, "@")
			project, err := db.GetProjectByName(projectName)
			if err != nil {
				return fmt.Errorf("failed to get project: %w", err)
			}
			if project == nil {
				fmt.Printf("No entries found for project @%s\n", projectName)
				return nil
			}
			opts.ProjectID = &project.ID
		} else if strings.HasPrefix(arg, "+") {
			tagName := strings.TrimPrefix(arg, "+")
			tag, err := db.GetTagByName(tagName)
			if err != nil {
				return fmt.Errorf("failed to get tag: %w", err)
			}
			if tag == nil {
				fmt.Printf("No entries found with tag +%s\n", tagName)
				return nil
			}
			opts.TagIDs = append(opts.TagIDs, tag.ID)
		}
	}

	// Parse date filters
	if logFrom != "" {
		t, err := time.Parse("2006-01-02", logFrom)
		if err != nil {
			return fmt.Errorf("invalid --from date (use YYYY-MM-DD): %w", err)
		}
		opts.From = &t
	}
	if logTo != "" {
		t, err := time.Parse("2006-01-02", logTo)
		if err != nil {
			return fmt.Errorf("invalid --to date (use YYYY-MM-DD): %w", err)
		}
		// Add a day to include the entire 'to' date
		t = t.Add(24 * time.Hour)
		opts.To = &t
	}

	entries, err := db.ListEntries(opts)
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No entries found")
		return nil
	}

	printEntriesTable(entries)
	return nil
}

// printEntriesTable formats and prints a table of entries to the console.
//
// It uses [tablewriter.Writer] to create a well-structured table displaying key details of each [model.Entry].
// The columns include "ID", "Project", "Title", "Duration", "Tags", and "Date". The function adjusts formatting
// dynamically, truncates titles longer than 30 characters, and appends indicators "*" or "~" to the duration
// for running or paused statuses respectively.
//
// entries is a slice of [model.Entry] objects, each representing a time-tracking entry with relevant metadata.
// The function reads specific attributes such as ID, project name, title, duration, tags, and start time.
//
// Prints the resulting table to standard output with additional symbols included:
//   - "*" appended to the duration for running entries.
//   - "~" appended to the duration for paused entries.
//
// This function ensures alignment, removes unnecessary table borders, and disables text wrapping for readability.
func printEntriesTable(entries []model.Entry) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Project", "Title", "Duration", "Tags", "Date"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)
	table.SetAutoWrapText(false)

	for _, e := range entries {
		duration := e.Duration()
		durationStr := formatDurationShort(duration)
		if e.Status == model.StatusRunning {
			durationStr += "*"
		} else if e.Status == model.StatusPaused {
			durationStr += "~"
		}

		tags := make([]string, len(e.Tags))
		for i, t := range e.Tags {
			tags[i] = t.Name
		}

		title := e.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}

		table.Append([]string{
			e.ID,
			"@" + e.Project.Name,
			title,
			durationStr,
			strings.Join(tags, ", "),
			e.StartTime.Format("2006-01-02 15:04"),
		})
	}

	table.Render()
	fmt.Println("\n* = running, ~ = paused")
}
