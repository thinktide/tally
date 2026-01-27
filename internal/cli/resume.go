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

var resumeFrom string

var resumeCmd = &cobra.Command{
	Use:   "resume [@project]",
	Short: "Resume a paused or stopped timer",
	Long: `Resume a paused timer or reopen a stopped entry.

If a timer is paused, it resumes immediately.
If the last entry is stopped, it shows details and asks for confirmation.
Reopening a stopped entry creates a pause from the stop time to now.

With @project, resumes the most recent task from that project:
  - If the latest entry matches the project, reopens it as usual.
  - Otherwise, clones the most recent entry for that project (same title
    and tags) into a new entry starting now (or at the -f time).

Use -f to specify a custom start/resume time.`,
	RunE: runResume,
}

func init() {
	resumeCmd.Flags().StringVarP(&resumeFrom, "from", "f", "", "Resume start time (HH:MM or YYYY-MM-DD HH:MM:SS)")
}

// parseResumeArgs extracts an optional @project from the arguments.
func parseResumeArgs(args []string) (string, error) {
	var project string
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			if project != "" {
				return "", fmt.Errorf("multiple projects specified")
			}
			project = strings.TrimPrefix(arg, "@")
		} else {
			return "", fmt.Errorf("unexpected argument: %s", arg)
		}
	}
	return project, nil
}

func runResume(cmd *cobra.Command, args []string) error {
	projectFilter, err := parseResumeArgs(args)
	if err != nil {
		return err
	}

	// Determine the resume/start time
	var startTime time.Time
	if resumeFrom != "" {
		startTime, err = parseTimeInput(resumeFrom)
		if err != nil {
			return err
		}
	} else {
		startTime = time.Now()
	}

	// If @project is specified, use project-specific resume logic
	if projectFilter != "" {
		return resumeProject(projectFilter, startTime)
	}

	// Default behavior: resume paused/stopped entry
	return resumeDefault(startTime)
}

func resumeProject(projectName string, startTime time.Time) error {
	project, err := db.GetProjectByName(projectName)
	if err != nil {
		return fmt.Errorf("failed to look up project: %w", err)
	}
	if project == nil {
		return fmt.Errorf("no entries found for project @%s", projectName)
	}

	// Stop any currently running/paused entry first
	running, err := db.GetRunningEntry()
	if err != nil {
		return fmt.Errorf("failed to get running entry: %w", err)
	}
	if running != nil {
		if err := db.StopEntry(running.ID); err != nil {
			return fmt.Errorf("failed to stop current entry: %w", err)
		}
		running, err = db.GetEntryByID(running.ID)
		if err != nil {
			return fmt.Errorf("failed to reload stopped entry: %w", err)
		}
		duration := running.Duration()
		fmt.Printf("Stopped timer for @%s", running.Project.Name)
		if running.Title != "" {
			fmt.Printf(": %s", running.Title)
		}
		fmt.Printf(" [%s]\n", formatDuration(duration))
	}

	// Check if the most recent overall entry is already from this project
	lastEntry, err := db.GetLastEntry()
	if err != nil {
		return fmt.Errorf("failed to get last entry: %w", err)
	}

	if lastEntry != nil && lastEntry.ProjectID == project.ID {
		// The latest entry is from this project - reopen it (with confirmation)
		return reopenEntry(lastEntry, startTime)
	}

	// Find the most recent entry for this project
	projectEntry, err := db.GetLastEntryForProject(project.ID)
	if err != nil {
		return fmt.Errorf("failed to get last entry for project: %w", err)
	}
	if projectEntry == nil {
		return fmt.Errorf("no entries found for project @%s", projectName)
	}

	// Clone: create a new entry with the same title and tags
	tagIDs := make([]string, len(projectEntry.Tags))
	for i, t := range projectEntry.Tags {
		tagIDs[i] = t.ID
	}

	newEntry, err := db.CreateEntryAt(project.ID, projectEntry.Title, tagIDs, startTime)
	if err != nil {
		return fmt.Errorf("failed to create entry: %w", err)
	}

	fmt.Printf("Resumed @%s", projectName)
	if newEntry.Title != "" {
		fmt.Printf(": %s", newEntry.Title)
	}
	if len(newEntry.Tags) > 0 {
		fmt.Printf(" %s", formatTagsFromModel(newEntry.Tags))
	}
	fmt.Println()
	return nil
}

func reopenEntry(entry *model.Entry, startTime time.Time) error {
	if entry.Status != model.StatusStopped {
		fmt.Println("No timer to resume")
		return nil
	}

	// Show entry details and ask for confirmation
	fmt.Println("Last entry:")
	fmt.Printf("  Project: @%s\n", entry.Project.Name)
	if entry.Title != "" {
		fmt.Printf("  Title:   %s\n", entry.Title)
	}
	if len(entry.Tags) > 0 {
		fmt.Printf("  Tags:    %s\n", formatTagsFromModel(entry.Tags))
	}
	fmt.Printf("  Started: %s\n", entry.StartTime.Format("2006-01-02 15:04:05"))
	if entry.EndTime != nil {
		fmt.Printf("  Stopped: %s\n", entry.EndTime.Format("2006-01-02 15:04:05"))
		gap := time.Since(*entry.EndTime)
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

	// Create pause for the gap
	if entry.EndTime != nil {
		_, err = db.CreatePause(entry.ID, *entry.EndTime, &startTime, "Manual")
		if err != nil {
			return fmt.Errorf("failed to create pause: %w", err)
		}
	}

	if err := db.ReopenEntry(entry.ID); err != nil {
		return fmt.Errorf("failed to reopen entry: %w", err)
	}

	fmt.Printf("Reopened timer for @%s", entry.Project.Name)
	if entry.Title != "" {
		fmt.Printf(": %s", entry.Title)
	}
	fmt.Println()

	return nil
}

func resumeDefault(startTime time.Time) error {
	// First check for running/paused entry
	entry, err := db.GetRunningEntry()
	if err != nil {
		return fmt.Errorf("failed to get running entry: %w", err)
	}

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

	return reopenEntry(lastEntry, startTime)
}
