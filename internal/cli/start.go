package cli

import (
	"fmt"
	"strings"

	"github.com/jdecarlo/tally/internal/db"
	"github.com/jdecarlo/tally/internal/model"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start @project [\"title\"] [+tag1] [+tag2]...",
	Short: "Start a new time entry",
	Long: `Start a new time entry for a project.

Examples:
  tally start @work
  tally start @work "Fixing bugs"
  tally start @work "Fixing bugs" +backend +urgent
  tally start @personal +coding`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	// Check if there's already a running entry
	running, err := db.GetRunningEntry()
	if err != nil {
		return fmt.Errorf("failed to check running entry: %w", err)
	}
	if running != nil {
		fmt.Println("Timer already running:")
		printStatus(running)
		return nil
	}

	// Parse arguments
	projectName, title, tagNames, err := parseStartArgs(args)
	if err != nil {
		return err
	}

	// Get or create project
	project, err := db.GetOrCreateProject(projectName)
	if err != nil {
		return fmt.Errorf("failed to get/create project: %w", err)
	}

	// Get or create tags
	var tagIDs []string
	for _, name := range tagNames {
		tag, err := db.GetOrCreateTag(name)
		if err != nil {
			return fmt.Errorf("failed to get/create tag '%s': %w", name, err)
		}
		tagIDs = append(tagIDs, tag.ID)
	}

	// Create entry
	entry, err := db.CreateEntry(project.ID, title, tagIDs)
	if err != nil {
		return fmt.Errorf("failed to create entry: %w", err)
	}

	entry.Project = project

	fmt.Printf("Started timer for @%s", project.Name)
	if title != "" {
		fmt.Printf(": %s", title)
	}
	if len(tagNames) > 0 {
		fmt.Printf(" [%s]", formatTags(tagNames))
	}
	fmt.Println()

	return nil
}

func parseStartArgs(args []string) (project, title string, tags []string, err error) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			if project != "" {
				err = fmt.Errorf("multiple projects specified")
				return
			}
			project = strings.TrimPrefix(arg, "@")
		} else if strings.HasPrefix(arg, "+") {
			tags = append(tags, strings.TrimPrefix(arg, "+"))
		} else {
			// Treat as title
			if title != "" {
				title += " " + arg
			} else {
				title = arg
			}
		}
	}

	if project == "" {
		err = fmt.Errorf("project is required (use @projectname)")
		return
	}

	return
}

func formatTags(tags []string) string {
	result := make([]string, len(tags))
	for i, t := range tags {
		result[i] = "+" + t
	}
	return strings.Join(result, " ")
}

func formatTagsFromModel(tags []model.Tag) string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return formatTags(names)
}
