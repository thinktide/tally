package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	logLimit int
	logFrom  string
	logTo    string
)

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

func init() {
	logCmd.Flags().IntVarP(&logLimit, "limit", "n", 10, "Number of entries to show")
	logCmd.Flags().StringVar(&logFrom, "from", "", "Start date (YYYY-MM-DD)")
	logCmd.Flags().StringVar(&logTo, "to", "", "End date (YYYY-MM-DD)")
}

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
			tags[i] = "+" + t.Name
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
			strings.Join(tags, " "),
			e.StartTime.Format("2006-01-02 15:04"),
		})
	}

	table.Render()
	fmt.Println("\n* = running, ~ = paused")
}
