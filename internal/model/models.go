package model

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewULID generates a new ULID
func NewULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type EntryStatus string

const (
	StatusRunning EntryStatus = "running"
	StatusPaused  EntryStatus = "paused"
	StatusStopped EntryStatus = "stopped"
)

type Entry struct {
	ID        string      `json:"id"`
	ProjectID string      `json:"project_id"`
	Project   *Project    `json:"project,omitempty"`
	Title     string      `json:"title"`
	StartTime time.Time   `json:"start_time"`
	EndTime   *time.Time  `json:"end_time,omitempty"`
	Status    EntryStatus `json:"status"`
	Tags      []Tag       `json:"tags,omitempty"`
	Pauses    []Pause     `json:"pauses,omitempty"`
}

// Duration calculates the actual working duration excluding pauses
func (e *Entry) Duration() time.Duration {
	endTime := time.Now()
	if e.EndTime != nil {
		endTime = *e.EndTime
	}

	total := endTime.Sub(e.StartTime)

	// Subtract pause durations
	for _, p := range e.Pauses {
		if p.ResumeTime != nil {
			total -= p.ResumeTime.Sub(p.PauseTime)
		} else if e.Status == StatusPaused {
			// Currently paused, subtract time from pause start to now
			total -= time.Since(p.PauseTime)
		}
	}

	return total
}

type Pause struct {
	ID         string     `json:"id"`
	EntryID    string     `json:"entry_id"`
	PauseTime  time.Time  `json:"pause_time"`
	ResumeTime *time.Time `json:"resume_time,omitempty"`
	Reason     string     `json:"reason"`
}

// Duration returns the duration of this pause
func (p *Pause) Duration() time.Duration {
	if p.ResumeTime != nil {
		return p.ResumeTime.Sub(p.PauseTime)
	}
	return time.Since(p.PauseTime)
}

type EntryTag struct {
	EntryID string `json:"entry_id"`
	TagID   string `json:"tag_id"`
}

// ReportEntry is used for report output
type ReportEntry struct {
	Entry
	ProjectName string        `json:"project_name"`
	TagNames    []string      `json:"tag_names"`
	Duration    time.Duration `json:"duration"`
}

// ReportSummary contains aggregated report data
type ReportSummary struct {
	TotalDuration    time.Duration            `json:"total_duration"`
	ByProject        map[string]time.Duration `json:"by_project"`
	ByTag            map[string]time.Duration `json:"by_tag"`
	Entries          []ReportEntry            `json:"entries"`
	Period           string                   `json:"period"`
	StartDate        time.Time                `json:"start_date"`
	EndDate          time.Time                `json:"end_date"`
}
