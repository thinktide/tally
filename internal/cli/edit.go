package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
)

// editCmd provides functionality to edit a time entry in the user's default editor (defaults to vim) as a JSON file.
//
// The command can edit either the most recent entry or a specified entry by its ID.
// It accepts at most one argument, which is the ID of the entry to be edited.
// If no ID is provided, the most recent entry is edited.
//
// The editor to be used is determined using the $EDITOR environment variable.
// If the $EDITOR variable is not set, "vim" is used as the default editor.
//
// On successful execution, the command updates the selected entry and its related data, such as tags and pauses. Possible
// error cases include invalid or missing entry data, parsing errors, or issues with external dependencies such as the editor.
var editCmd = &cobra.Command{
	Use:   "edit [id]",
	Short: "Edit a time entry",
	Long: `Edit a time entry in your editor. Without an ID, edits the most recent entry.

Examples:
  tally edit        # Edit most recent entry
  tally edit 42     # Edit entry with ID 42

Opens the entry as JSON in $EDITOR (defaults to vim).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEdit,
}

// editableEntry represents an entry that can be modified with enriched details about its state and associated metadata.
//
// This type includes fields to store information such as the entry's ID, associated project, title, tags, time durations,
// and operational status.
//
// The pauses field allows for tracking periods of interruption within the entry, represented as a slice of [editPause].
// When serialized to JSON, fields such as EndTime and Pauses are omitted if their values are empty or nil.
//
// Fields:
//   - ID: The unique identifier for the entry.
//   - Project: The name of the project the entry is associated with.
//   - Title: A short description of the entry.
//   - Tags: A list of tags categorizing the entry.
//   - StartTime: The starting time of the entry in a formatted string (e.g., "2006-01-02 15:04:05").
//   - EndTime: The optional ending time of the entry in a formatted string (if available).
//   - Status: The current status of the entry, commonly used to track progress or state.
//   - Pauses: A slice of [editPause] indicating pauses and associated details within the entry's timeline.
type editableEntry struct {
	ID        string      `json:"id"`
	Project   string      `json:"project"`
	Title     string      `json:"title"`
	Tags      []string    `json:"tags"`
	StartTime string      `json:"start_time"`
	EndTime   string      `json:"end_time,omitempty"`
	Status    string      `json:"status"`
	Pauses    []editPause `json:"pauses,omitempty"`
}

// editPause represents a pause period within an editable entry structure.
//
// This type captures the details of a paused interval, such as the unique ID, the time the pause started,
// and optionally the time the pause ended. The reason for the pause can also be documented.
//
// - ID uniquely identifies the pause.
// - PauseTime specifies when the pause began.
// - ResumeTime optionally specifies when the pause ended.
// - Reason provides a brief explanation or context for the pause.
type editPause struct {
	ID         string `json:"id"`
	PauseTime  string `json:"pause_time"`
	ResumeTime string `json:"resume_time,omitempty"`
	Reason     string `json:"reason"`
}

// runEdit edits an existing entry identified by its ID or the most recent entry if no ID is provided.
//
// If no arguments are passed, the function attempts to retrieve the most recent entry from the database.
// The user edits the entry details in a temporary file using the editor specified by the "EDITOR" environment variable.
// If the editor is not defined, "vim" is used as the default. Updates are validated and saved back to the database.
//
//   - cmd: The [cobra.Command] instance that executes this function.
//   - args: An optional list of arguments where the first argument is the entry ID to edit.
//
// Returns an error if none of the following operations succeed:
//   - Retrieving the entry from the database.
//   - Creating or writing to a temporary file.
//   - Parsing the entry data from the edited file.
//   - Updating the database record, including its associated projects, tags, and pauses.
//
// Validation errors, like invalid timestamps or inconsistent pause information, will also result in returned errors.
func runEdit(cmd *cobra.Command, args []string) error {
	var entryID string

	if len(args) == 0 {
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

	// Build editable structure
	tags := make([]string, len(entry.Tags))
	for i, t := range entry.Tags {
		tags[i] = t.Name
	}

	editable := editableEntry{
		ID:        entry.ID,
		Project:   entry.Project.Name,
		Title:     entry.Title,
		Tags:      tags,
		StartTime: entry.StartTime.Format("2006-01-02 15:04:05"),
		Status:    string(entry.Status),
	}

	if entry.EndTime != nil {
		editable.EndTime = entry.EndTime.Format("2006-01-02 15:04:05")
	}

	for _, p := range entry.Pauses {
		ep := editPause{
			ID:        p.ID,
			PauseTime: p.PauseTime.Format("2006-01-02 15:04:05"),
			Reason:    p.Reason,
		}
		if p.ResumeTime != nil {
			ep.ResumeTime = p.ResumeTime.Format("2006-01-02 15:04:05")
		}
		editable.Pauses = append(editable.Pauses, ep)
	}

	// Write to temp file
	tmpfile, err := os.CreateTemp("", "tally-edit-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpfile.Name()
	defer os.Remove(tmpPath)

	encoder := json.NewEncoder(tmpfile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(editable); err != nil {
		tmpfile.Close()
		return fmt.Errorf("failed to write JSON: %w", err)
	}
	tmpfile.Close()

	// Get editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Open editor
	editCmd := exec.Command(editor, tmpPath)
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr

	if err := editCmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Read back
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read temp file: %w", err)
	}

	var updated editableEntry
	if err := json.Unmarshal(data, &updated); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Parse times
	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", updated.StartTime, time.Local)
	if err != nil {
		return fmt.Errorf("invalid start_time format: %w", err)
	}

	var endTime *time.Time
	if updated.EndTime != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", updated.EndTime, time.Local)
		if err != nil {
			return fmt.Errorf("invalid end_time format: %w", err)
		}
		endTime = &t
	}

	// Validate
	if endTime != nil && endTime.Before(startTime) {
		return fmt.Errorf("end_time cannot be before start_time")
	}

	// Get or create project
	project, err := db.GetOrCreateProject(updated.Project)
	if err != nil {
		return fmt.Errorf("failed to get/create project: %w", err)
	}

	// Get or create tags
	var tagIDs []string
	for _, name := range updated.Tags {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tag, err := db.GetOrCreateTag(name)
		if err != nil {
			return fmt.Errorf("failed to get/create tag: %w", err)
		}
		tagIDs = append(tagIDs, tag.ID)
	}

	// Update entry
	if err := db.UpdateEntry(entryID, project.ID, updated.Title, &startTime, endTime, tagIDs); err != nil {
		return fmt.Errorf("failed to update entry: %w", err)
	}

	// Handle pauses - build map of existing pause IDs
	existingPauses := make(map[string]bool)
	for _, p := range entry.Pauses {
		existingPauses[p.ID] = true
	}

	// Update or track pauses from edited JSON
	updatedPauses := make(map[string]bool)
	for _, p := range updated.Pauses {
		pauseTime, err := time.ParseInLocation("2006-01-02 15:04:05", p.PauseTime, time.Local)
		if err != nil {
			return fmt.Errorf("invalid pause_time format: %w", err)
		}

		var resumeTime *time.Time
		if p.ResumeTime != "" {
			t, err := time.ParseInLocation("2006-01-02 15:04:05", p.ResumeTime, time.Local)
			if err != nil {
				return fmt.Errorf("invalid resume_time format: %w", err)
			}
			resumeTime = &t
		}

		if p.ID == "" {
			// Create new pause
			reason := p.Reason
			if reason == "" {
				reason = "Manual"
			}
			_, err := db.CreatePause(entryID, pauseTime, resumeTime, reason)
			if err != nil {
				return fmt.Errorf("failed to create pause: %w", err)
			}
		} else {
			// Update existing pause
			updatedPauses[p.ID] = true
			if err := db.UpdatePause(p.ID, pauseTime, resumeTime); err != nil {
				return fmt.Errorf("failed to update pause: %w", err)
			}
		}
	}

	// Delete pauses that were removed from JSON
	for _, p := range entry.Pauses {
		if !updatedPauses[p.ID] {
			if err := db.DeletePause(p.ID); err != nil {
				return fmt.Errorf("failed to delete pause: %w", err)
			}
		}
	}

	fmt.Println("Entry updated successfully")
	return nil
}
