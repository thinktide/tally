package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thinktide/tally/internal/db"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit [id]",
	Short: "Edit a time entry",
	Long: `Edit a time entry. Without an ID, edits the most recent entry.

Examples:
  tally edit        # Edit most recent entry
  tally edit 42     # Edit entry with ID 42`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEdit,
}

func runEdit(cmd *cobra.Command, args []string) error {
	var entryID string

	if len(args) == 0 {
		// Edit most recent entry
		entry, err := db.GetLastEntry()
		if err != nil {
			return fmt.Errorf("failed to get last entry: %w", err)
		}
		if entry == nil {
			fmt.Println("No entries to edit")
			return nil
		}
		entryID = entry.ID
	} else {
		entryID = args[0]
	}

	entry, err := db.GetEntryByID(entryID)
	if err != nil {
		return fmt.Errorf("entry not found: %w", err)
	}

	fmt.Printf("Editing entry %s\n", entry.ID)
	fmt.Println("Press Enter to keep current value, or type a new value.\n")

	reader := bufio.NewReader(os.Stdin)

	// Project
	fmt.Printf("Project [@%s]: ", entry.Project.Name)
	projectInput, _ := reader.ReadString('\n')
	projectInput = strings.TrimSpace(projectInput)

	projectID := entry.ProjectID
	if projectInput != "" {
		projectName := strings.TrimPrefix(projectInput, "@")
		project, err := db.GetOrCreateProject(projectName)
		if err != nil {
			return fmt.Errorf("failed to get/create project: %w", err)
		}
		projectID = project.ID
	}

	// Title
	currentTitle := entry.Title
	if currentTitle == "" {
		currentTitle = "(none)"
	}
	fmt.Printf("Title [%s]: ", currentTitle)
	titleInput, _ := reader.ReadString('\n')
	titleInput = strings.TrimSpace(titleInput)

	title := entry.Title
	if titleInput != "" {
		if titleInput == "-" {
			title = ""
		} else {
			title = titleInput
		}
	}

	// Tags
	currentTags := make([]string, len(entry.Tags))
	for i, t := range entry.Tags {
		currentTags[i] = "+" + t.Name
	}
	currentTagsStr := strings.Join(currentTags, " ")
	if currentTagsStr == "" {
		currentTagsStr = "(none)"
	}
	fmt.Printf("Tags [%s]: ", currentTagsStr)
	tagsInput, _ := reader.ReadString('\n')
	tagsInput = strings.TrimSpace(tagsInput)

	var tagIDs []string
	if tagsInput != "" {
		if tagsInput == "-" {
			tagIDs = []string{}
		} else {
			tagNames := strings.Fields(tagsInput)
			for _, name := range tagNames {
				name = strings.TrimPrefix(name, "+")
				tag, err := db.GetOrCreateTag(name)
				if err != nil {
					return fmt.Errorf("failed to get/create tag: %w", err)
				}
				tagIDs = append(tagIDs, tag.ID)
			}
		}
	} else {
		for _, t := range entry.Tags {
			tagIDs = append(tagIDs, t.ID)
		}
	}

	// Start time
	fmt.Printf("Start time [%s]: ", entry.StartTime.Format("2006-01-02 15:04:05"))
	startInput, _ := reader.ReadString('\n')
	startInput = strings.TrimSpace(startInput)

	var startTime *time.Time = &entry.StartTime
	if startInput != "" {
		t, err := parseTime(startInput)
		if err != nil {
			return fmt.Errorf("invalid start time: %w", err)
		}
		startTime = &t
	}

	// End time (only for stopped entries)
	var endTime *time.Time = entry.EndTime
	if entry.EndTime != nil {
		fmt.Printf("End time [%s]: ", entry.EndTime.Format("2006-01-02 15:04:05"))
		endInput, _ := reader.ReadString('\n')
		endInput = strings.TrimSpace(endInput)

		if endInput != "" {
			t, err := parseTime(endInput)
			if err != nil {
				return fmt.Errorf("invalid end time: %w", err)
			}
			endTime = &t
		}
	}

	// Validate
	if endTime != nil && startTime != nil && endTime.Before(*startTime) {
		return fmt.Errorf("end time cannot be before start time")
	}

	// Update
	if err := db.UpdateEntry(entryID, projectID, title, startTime, endTime, tagIDs); err != nil {
		return fmt.Errorf("failed to update entry: %w", err)
	}

	// Handle pauses
	if len(entry.Pauses) > 0 {
		fmt.Printf("\nPauses (%d):\n", len(entry.Pauses))
		for i, p := range entry.Pauses {
			resumeStr := "(ongoing)"
			if p.ResumeTime != nil {
				resumeStr = p.ResumeTime.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("  %d. %s - %s [%s]\n", i+1,
				p.PauseTime.Format("2006-01-02 15:04:05"),
				resumeStr,
				formatDuration(p.Duration()))
		}
		fmt.Println()

		for i, p := range entry.Pauses {
			fmt.Printf("Pause %d - Edit (e), Delete (d), or Enter to skip: ", i+1)
			action, _ := reader.ReadString('\n')
			action = strings.TrimSpace(strings.ToLower(action))

			switch action {
			case "d", "delete":
				if err := db.DeletePause(p.ID); err != nil {
					return fmt.Errorf("failed to delete pause: %w", err)
				}
				fmt.Println("  Pause deleted")

			case "e", "edit":
				// Edit pause start
				fmt.Printf("  Pause start [%s]: ", p.PauseTime.Format("2006-01-02 15:04:05"))
				pauseStartInput, _ := reader.ReadString('\n')
				pauseStartInput = strings.TrimSpace(pauseStartInput)

				pauseStart := p.PauseTime
				if pauseStartInput != "" {
					t, err := parseTime(pauseStartInput)
					if err != nil {
						return fmt.Errorf("invalid pause start time: %w", err)
					}
					pauseStart = t
				}

				// Edit pause end
				var pauseEnd *time.Time = p.ResumeTime
				if p.ResumeTime != nil {
					fmt.Printf("  Pause end [%s]: ", p.ResumeTime.Format("2006-01-02 15:04:05"))
				} else {
					fmt.Print("  Pause end [(ongoing)]: ")
				}
				pauseEndInput, _ := reader.ReadString('\n')
				pauseEndInput = strings.TrimSpace(pauseEndInput)

				if pauseEndInput != "" {
					t, err := parseTime(pauseEndInput)
					if err != nil {
						return fmt.Errorf("invalid pause end time: %w", err)
					}
					pauseEnd = &t
				}

				if err := db.UpdatePause(p.ID, pauseStart, pauseEnd); err != nil {
					return fmt.Errorf("failed to update pause: %w", err)
				}
				fmt.Println("  Pause updated")
			}
		}
	}

	fmt.Println("\nEntry updated successfully")
	return nil
}

func parseTime(s string) (time.Time, error) {
	// Try various formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"15:04:05",
		"15:04",
	}

	for _, format := range formats {
		t, err := time.ParseInLocation(format, s, time.Local)
		if err == nil {
			// If only time was provided, use today's date
			if len(s) <= 8 {
				now := time.Now()
				t = time.Date(now.Year(), now.Month(), now.Day(),
					t.Hour(), t.Minute(), t.Second(), 0, time.Local)
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized time format (use YYYY-MM-DD HH:MM:SS or HH:MM)")
}
