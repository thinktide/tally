package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import time entries from other tools",
	Long: `Import time entries from other time tracking tools.

Supported formats:
  watson    Watson CSV export (watson log --all --csv > watson.csv)

Examples:
  watson log --all --csv | tally import --format watson -
  tally import --format watson watson.csv`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

var importFormat string
var importDryRun bool

func init() {
	importCmd.Flags().StringVarP(&importFormat, "format", "f", "watson", "Import format (watson)")
	importCmd.Flags().BoolVarP(&importDryRun, "dry-run", "n", false, "Preview import without saving")
}

func runImport(cmd *cobra.Command, args []string) error {
	filename := args[0]

	var reader io.Reader
	if filename == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		reader = file
	}

	switch importFormat {
	case "watson":
		return importWatson(reader)
	default:
		return fmt.Errorf("unknown format: %s", importFormat)
	}
}

func importWatson(reader io.Reader) error {
	csvReader := csv.NewReader(reader)

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Validate header
	expectedHeader := []string{"id", "start", "stop", "project", "tags"}
	if len(header) != len(expectedHeader) {
		return fmt.Errorf("invalid header: expected %v", expectedHeader)
	}

	var imported, skipped int
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read record: %w", err)
		}

		if len(record) != 5 {
			skipped++
			continue
		}

		// Parse record
		// id, start, stop, project, tags
		startTime, err := time.ParseInLocation("2006-01-02 15:04:05", record[1], time.Local)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping: invalid start time %q: %v\n", record[1], err)
			skipped++
			continue
		}

		stopTime, err := time.ParseInLocation("2006-01-02 15:04:05", record[2], time.Local)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping: invalid stop time %q: %v\n", record[2], err)
			skipped++
			continue
		}

		projectName := record[3]
		tagsStr := record[4]

		// Parse tags (comma-separated, may have spaces)
		var tagNames []string
		if tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tagNames = append(tagNames, t)
				}
			}
		}

		if importDryRun {
			fmt.Printf("Would import: @%s %s [%s] %s - %s\n",
				projectName,
				strings.Join(tagNames, " +"),
				formatDuration(stopTime.Sub(startTime)),
				startTime.Format("2006-01-02 15:04"),
				stopTime.Format("15:04"))
			imported++
			continue
		}

		// Create project
		project, err := db.GetOrCreateProject(projectName)
		if err != nil {
			return fmt.Errorf("failed to create project %q: %w", projectName, err)
		}

		// Create tags
		var tagIDs []string
		for _, name := range tagNames {
			tag, err := db.GetOrCreateTag(name)
			if err != nil {
				return fmt.Errorf("failed to create tag %q: %w", name, err)
			}
			tagIDs = append(tagIDs, tag.ID)
		}

		// Create entry directly with start/stop times
		if err := createImportedEntry(project.ID, "", startTime, stopTime, tagIDs); err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		imported++
	}

	if importDryRun {
		fmt.Printf("\nDry run: would import %d entries (%d skipped)\n", imported, skipped)
	} else {
		fmt.Printf("Imported %d entries (%d skipped)\n", imported, skipped)
	}

	return nil
}

func createImportedEntry(projectID string, title string, startTime, endTime time.Time, tagIDs []string) error {
	entryID := model.NewULID()

	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO entries (id, project_id, title, start_time, end_time, status) VALUES (?, ?, ?, ?, ?, ?)",
		entryID, projectID, title, startTime, endTime, model.StatusStopped)
	if err != nil {
		return err
	}

	for _, tagID := range tagIDs {
		_, err = tx.Exec("INSERT INTO entry_tags (entry_id, tag_id) VALUES (?, ?)", entryID, tagID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
