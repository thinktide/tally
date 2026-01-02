package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
)

// startCmd initializes the "start" command for creating a new time entry for a specific project.
//
// This command allows users to start a time entry with the specified project, optional title, and tags.
//
//   - args[0]: The name of the project prefixed with "@". This argument is mandatory.
//   - Subsequent arguments can include an optional title in quotes and one or more tags prefixed with "+".
//
// The command ensures that:
//   - Only one timer can run at a time.
//   - A new project or tag is created automatically if it does not exist.
//
// Returns an error if the provided arguments are invalid, or if there is an issue creating the time entry.
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

// runStart initializes and starts a new time entry for a specified project. It validates if an entry is already running.
//
// If there is an ongoing entry, it prints its status and exits. Otherwise, it processes the input arguments to extract the project name,
// title, and associated tags. The project and tags are retrieved or created if they do not already exist, and a new entry is created.
//
// - cmd: The current [cobra.Command] being executed.
// - args: The command-line arguments provided for the "start" operation.
//
// Returns an error if any of the following occur:
//   - Retrieving or creating a project or tag fails.
//   - Parsing the arguments fails.
//   - A database or application-level failure occurs during entry creation.
//
// If successful, details of the started timer are printed to the console.
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

// parseStartArgs parses command-line arguments to extract a project name, title, and tags.
//
// It expects the following format:
//   - A single project name prefixed with "@" (e.g., "@projectname").
//   - Tags prefixed with "+" (e.g., "+tag1").
//   - Remaining arguments are treated as the entry title, allowing multiple words.
//
// If multiple projects are specified, it returns an error. A project name is required.
//
// args:
//   - An array of strings representing the command-line arguments.
//
// Returns:
//   - project: The project name extracted from the arguments.
//   - title: The constructed title from remaining positional arguments, if any.
//   - tags: A slice of tag names extracted from the arguments.
//   - err: An error if validation fails, such as missing project or duplicate project.
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

// formatTags formats a slice of tags as a single string with each tag prefixed by a "+" and separated by a space.
//
// tags is a slice of strings representing individual tag names.
//
// Returns a single string with formatted tags concatenated with a space between them.
func formatTags(tags []string) string {
	result := make([]string, len(tags))
	for i, t := range tags {
		result[i] = "+" + t
	}
	return strings.Join(result, " ")
}

// formatTagsFromModel converts a slice of [model.Tag] into a formatted string of tag names prefixed with "+" and separated by spaces.
//
// The function extracts the Name field from each [model.Tag] in the input slice tags and passes the resulting slice of names
// to the [formatTags] function for formatting. This process ensures that the output is a properly formatted string representation
// of the tags.
//
// tags:
//   - A slice of [model.Tag], each containing metadata such as name, ID, and creation time.
//
// Returns a single string where each tag name is prefixed with "+" and all names are concatenated with spaces.
func formatTagsFromModel(tags []model.Tag) string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return formatTags(names)
}
