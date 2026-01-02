package db

import (
	"database/sql"
	"time"

	"github.com/jdecarlo/tally/internal/model"
)

// Project operations

func GetOrCreateProject(name string) (*model.Project, error) {
	// Try to get existing project
	var p model.Project
	err := DB.QueryRow("SELECT id, name, created_at FROM projects WHERE name = ?", name).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err == nil {
		return &p, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new project with ULID
	id := model.NewULID()
	now := time.Now()
	_, err = DB.Exec("INSERT INTO projects (id, name, created_at) VALUES (?, ?, ?)", id, name, now)
	if err != nil {
		return nil, err
	}

	return &model.Project{
		ID:        id,
		Name:      name,
		CreatedAt: now,
	}, nil
}

func GetProjectByID(id string) (*model.Project, error) {
	var p model.Project
	err := DB.QueryRow("SELECT id, name, created_at FROM projects WHERE id = ?", id).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Tag operations

func GetOrCreateTag(name string) (*model.Tag, error) {
	var t model.Tag
	err := DB.QueryRow("SELECT id, name, created_at FROM tags WHERE name = ?", name).
		Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err == nil {
		return &t, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	id := model.NewULID()
	now := time.Now()
	_, err = DB.Exec("INSERT INTO tags (id, name, created_at) VALUES (?, ?, ?)", id, name, now)
	if err != nil {
		return nil, err
	}

	return &model.Tag{
		ID:        id,
		Name:      name,
		CreatedAt: now,
	}, nil
}

func GetTagsForEntry(entryID string) ([]model.Tag, error) {
	rows, err := DB.Query(`
		SELECT t.id, t.name, t.created_at
		FROM tags t
		JOIN entry_tags et ON t.id = et.tag_id
		WHERE et.entry_id = ?`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// Entry operations

func CreateEntry(projectID string, title string, tagIDs []string) (*model.Entry, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	entryID := model.NewULID()
	now := time.Now()
	_, err = tx.Exec(
		"INSERT INTO entries (id, project_id, title, start_time, status) VALUES (?, ?, ?, ?, ?)",
		entryID, projectID, title, now, model.StatusRunning)
	if err != nil {
		return nil, err
	}

	// Add tags
	for _, tagID := range tagIDs {
		_, err = tx.Exec("INSERT INTO entry_tags (entry_id, tag_id) VALUES (?, ?)", entryID, tagID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.Entry{
		ID:        entryID,
		ProjectID: projectID,
		Title:     title,
		StartTime: now,
		Status:    model.StatusRunning,
	}, nil
}

func GetRunningEntry() (*model.Entry, error) {
	var e model.Entry
	var endTime sql.NullTime
	err := DB.QueryRow(`
		SELECT id, project_id, title, start_time, end_time, status
		FROM entries
		WHERE status IN ('running', 'paused')
		ORDER BY start_time DESC LIMIT 1`).
		Scan(&e.ID, &e.ProjectID, &e.Title, &e.StartTime, &endTime, &e.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if endTime.Valid {
		e.EndTime = &endTime.Time
	}

	// Load project
	project, err := GetProjectByID(e.ProjectID)
	if err != nil {
		return nil, err
	}
	e.Project = project

	// Load tags
	tags, err := GetTagsForEntry(e.ID)
	if err != nil {
		return nil, err
	}
	e.Tags = tags

	// Load pauses
	pauses, err := GetPausesForEntry(e.ID)
	if err != nil {
		return nil, err
	}
	e.Pauses = pauses

	return &e, nil
}

func GetEntryByID(id string) (*model.Entry, error) {
	var e model.Entry
	var endTime sql.NullTime
	err := DB.QueryRow(`
		SELECT id, project_id, title, start_time, end_time, status
		FROM entries WHERE id = ?`, id).
		Scan(&e.ID, &e.ProjectID, &e.Title, &e.StartTime, &endTime, &e.Status)
	if err != nil {
		return nil, err
	}
	if endTime.Valid {
		e.EndTime = &endTime.Time
	}

	project, err := GetProjectByID(e.ProjectID)
	if err != nil {
		return nil, err
	}
	e.Project = project

	tags, err := GetTagsForEntry(e.ID)
	if err != nil {
		return nil, err
	}
	e.Tags = tags

	pauses, err := GetPausesForEntry(e.ID)
	if err != nil {
		return nil, err
	}
	e.Pauses = pauses

	return &e, nil
}

func GetLastEntry() (*model.Entry, error) {
	var id string
	err := DB.QueryRow("SELECT id FROM entries ORDER BY start_time DESC LIMIT 1").Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return GetEntryByID(id)
}

func StopEntry(id string) error {
	now := time.Now()

	// Close any open pauses first
	_, err := DB.Exec("UPDATE pauses SET resume_time = ? WHERE entry_id = ? AND resume_time IS NULL", now, id)
	if err != nil {
		return err
	}

	_, err = DB.Exec("UPDATE entries SET end_time = ?, status = ? WHERE id = ?", now, model.StatusStopped, id)
	return err
}

func PauseEntry(id string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pauseID := model.NewULID()
	now := time.Now()

	_, err = tx.Exec("INSERT INTO pauses (id, entry_id, pause_time) VALUES (?, ?, ?)", pauseID, id, now)
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE entries SET status = ? WHERE id = ?", model.StatusPaused, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func ResumeEntry(id string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()

	_, err = tx.Exec("UPDATE pauses SET resume_time = ? WHERE entry_id = ? AND resume_time IS NULL", now, id)
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE entries SET status = ? WHERE id = ?", model.StatusRunning, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func UpdateEntry(id string, projectID string, title string, startTime, endTime *time.Time, tagIDs []string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if startTime != nil && endTime != nil {
		_, err = tx.Exec("UPDATE entries SET project_id = ?, title = ?, start_time = ?, end_time = ? WHERE id = ?",
			projectID, title, *startTime, *endTime, id)
	} else if startTime != nil {
		_, err = tx.Exec("UPDATE entries SET project_id = ?, title = ?, start_time = ? WHERE id = ?",
			projectID, title, *startTime, id)
	} else {
		_, err = tx.Exec("UPDATE entries SET project_id = ?, title = ? WHERE id = ?",
			projectID, title, id)
	}
	if err != nil {
		return err
	}

	// Update tags - remove old and add new
	_, err = tx.Exec("DELETE FROM entry_tags WHERE entry_id = ?", id)
	if err != nil {
		return err
	}

	for _, tagID := range tagIDs {
		_, err = tx.Exec("INSERT INTO entry_tags (entry_id, tag_id) VALUES (?, ?)", id, tagID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

type ListEntriesOptions struct {
	Limit     int
	ProjectID *string
	TagIDs    []string
	From      *time.Time
	To        *time.Time
}

func ListEntries(opts ListEntriesOptions) ([]model.Entry, error) {
	query := `
		SELECT DISTINCT e.id, e.project_id, e.title, e.start_time, e.end_time, e.status
		FROM entries e
		LEFT JOIN entry_tags et ON e.id = et.entry_id
		WHERE 1=1`
	args := []interface{}{}

	if opts.ProjectID != nil {
		query += " AND e.project_id = ?"
		args = append(args, *opts.ProjectID)
	}

	if len(opts.TagIDs) > 0 {
		query += " AND et.tag_id IN (?" + repeatString(",?", len(opts.TagIDs)-1) + ")"
		for _, id := range opts.TagIDs {
			args = append(args, id)
		}
	}

	if opts.From != nil {
		query += " AND e.start_time >= ?"
		args = append(args, *opts.From)
	}

	if opts.To != nil {
		query += " AND e.start_time < ?"
		args = append(args, *opts.To)
	}

	query += " ORDER BY e.start_time DESC"

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.Entry
	for rows.Next() {
		var e model.Entry
		var endTime sql.NullTime
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Title, &e.StartTime, &endTime, &e.Status); err != nil {
			return nil, err
		}
		if endTime.Valid {
			e.EndTime = &endTime.Time
		}

		project, err := GetProjectByID(e.ProjectID)
		if err != nil {
			return nil, err
		}
		e.Project = project

		tags, err := GetTagsForEntry(e.ID)
		if err != nil {
			return nil, err
		}
		e.Tags = tags

		pauses, err := GetPausesForEntry(e.ID)
		if err != nil {
			return nil, err
		}
		e.Pauses = pauses

		entries = append(entries, e)
	}

	return entries, rows.Err()
}

func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// Pause operations

func GetPausesForEntry(entryID string) ([]model.Pause, error) {
	rows, err := DB.Query(`
		SELECT id, entry_id, pause_time, resume_time
		FROM pauses
		WHERE entry_id = ?
		ORDER BY pause_time`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pauses []model.Pause
	for rows.Next() {
		var p model.Pause
		var resumeTime sql.NullTime
		if err := rows.Scan(&p.ID, &p.EntryID, &p.PauseTime, &resumeTime); err != nil {
			return nil, err
		}
		if resumeTime.Valid {
			p.ResumeTime = &resumeTime.Time
		}
		pauses = append(pauses, p)
	}
	return pauses, rows.Err()
}

// Activity tracking

func UpdateLastActivity() error {
	_, err := DB.Exec("UPDATE activity SET last_activity = datetime('now') WHERE id = 1")
	return err
}

func GetLastActivity() (*time.Time, error) {
	var t time.Time
	err := DB.QueryRow("SELECT last_activity FROM activity WHERE id = 1").Scan(&t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Config operations

func GetConfig(key string) (string, error) {
	var value string
	err := DB.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func SetConfig(key, value string) error {
	_, err := DB.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", key, value)
	return err
}

func ListConfig() (map[string]string, error) {
	rows, err := DB.Query("SELECT key, value FROM config")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		config[key] = value
	}
	return config, rows.Err()
}

// Project/Tag listing for reports

func GetProjectByName(name string) (*model.Project, error) {
	var p model.Project
	err := DB.QueryRow("SELECT id, name, created_at FROM projects WHERE name = ?", name).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetTagByName(name string) (*model.Tag, error) {
	var t model.Tag
	err := DB.QueryRow("SELECT id, name, created_at FROM tags WHERE name = ?", name).
		Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}
