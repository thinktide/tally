package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/thinktide/tally/internal/db"
	"github.com/spf13/cobra"
)

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

// editableEntry is the JSON structure for editing
type editableEntry struct {
	ID        string        `json:"id"`
	Project   string        `json:"project"`
	Title     string        `json:"title"`
	Tags      []string      `json:"tags"`
	StartTime string        `json:"start_time"`
	EndTime   string        `json:"end_time,omitempty"`
	Status    string        `json:"status"`
	Pauses    []editPause   `json:"pauses,omitempty"`
}

type editPause struct {
	ID         string `json:"id"`
	PauseTime  string `json:"pause_time"`
	ResumeTime string `json:"resume_time,omitempty"`
	Reason     string `json:"reason"`
}

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
		if p.ID == "" {
			continue
		}
		updatedPauses[p.ID] = true

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

		if err := db.UpdatePause(p.ID, pauseTime, resumeTime); err != nil {
			return fmt.Errorf("failed to update pause: %w", err)
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

