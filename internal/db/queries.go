package db

import (
	"database/sql"
	"time"

	"github.com/thinktide/tally/internal/model"
)

// Project operations

// GetOrCreateProject retrieves an existing project by its name or creates a new one if no such project exists.
//
// If a project with the specified name is found in the database, it is returned. If no project is found, a new project
// is created with a unique ID and current timestamp, then inserted into the database.
//
// - name: The name of the project to retrieve or create.
//
// Returns a pointer to a [model.Project] representing the retrieved or newly created project.
// Returns an error if a database operation fails or another unexpected issue occurs.
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

// GetProjectByID retrieves a [model.Project] by its unique identifier.
//
// The parameter id is the unique identifier of the project to retrieve.
// If a project with the specified id exists, it is returned.
//
// Errors are returned in the following cases:
//   - If the id does not correspond to any existing project, an SQL-specific error is returned.
//   - Any other errors that occur during the database query execution.
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

// GetOrCreateTag retrieves a tag by its name or creates a new one if it does not exist.
//
// If a tag with the specified name exists in the database, it returns the corresponding [model.Tag] along with a nil error.
//
// If no matching tag is found, a new [model.Tag] is inserted into the database with a unique ID and the current timestamp.
// The function then returns the newly created tag.
//
// In case of a database query or insertion failure, it returns a nil tag and the associated error.
//
//   - name: The name of the tag to retrieve or create.
//
// Returns:
//   - A pointer to a [model.Tag] representing the retrieved or newly created tag.
//   - An error, if any issue occurs during the retrieval or creation process.
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

// GetTagsForEntry retrieves all [model.Tag]s associated with a given entry specified by entryID.
//
// It performs a database query to fetch details of tags linked to the entry via the entry_tags table.
//
// entryID is the unique identifier of the entry whose tags need to be retrieved.
//
// Returns a slice of [model.Tag] containing the tags associated with the entry.
// If an error occurs during query execution, it returns nil and the error.
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

// CreateEntry creates a new entry for a project with the specified title and tags.
//
// This function starts a database transaction to insert a new entry into the `entries` table, including its associated
// tags into the `entry_tags` table. The entry is initialized with a unique identifier, the current timestamp, and a
// "running" status.
//
// Any errors during the transaction (e.g., during the insertion of tags) will result in a rollback and an error return.
//
// Parameters:
//   - projectID: The unique identifier for the project to which this entry belongs.
//   - title: A short description of the entry.
//   - tagIDs:
//     A slice of unique identifiers representing the tags to associate with this entry.
//
// Returns:
//   - A pointer to the created [model.Entry], which contains the entry's details, including its ID and status.
//   - An error if any issues occur during the database operation, such as issues with the transaction or database
//     constraints.
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

// GetRunningEntry retrieves the most recent time entry with a status of 'running' or 'paused'.
//
// It queries the database for the active entry, ordering by start time in descending order, and applies limitations to return
// only one result. If no such entry exists, it returns nil without error. If any database interaction fails, it returns an error.
//
// The function augments the retrieved entry with associated data, including its project, tags, and pauses:
//
//   - Project information is loaded via [GetProjectByID].
//   - Tag associations are retrieved using [GetTagsForEntry].
//   - Pause details are loaded using [GetPausesForEntry].
//
// Returns:
//   - A pointer to a complete [model.Entry] containing associated details if an active entry is found.
//   - Nil if no entry is running or paused.
//   - An error if any database queries or related function calls fail.
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

// GetEntryByID retrieves a time entry from the database based on its ID.
//
// The function queries the `entries` table to fetch the relevant entry's details, including project information,
// associated tags, and any pauses linked to the entry.
//
// - id: The unique identifier of the entry to retrieve.
//
// Returns a pointer to [model.Entry], fully populated with its project, tags, and pauses.
// Returns an error if the entry is not found or if any database operation fails, such as retrieving related data.
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

// GetLastEntry retrieves the most recent [model.Entry] from the database based on the latest start time.
//
// If no entries exist in the database, the function returns `nil` without an error.
// Any database-related issues encountered during the query or ID scan will result in an error.
//
// Returns:
//   - A pointer to the most recent [model.Entry], or `nil` if no entries are found.
//   - An `error` if the query or subsequent retrieval fails, with specific handling for `sql.ErrNoRows`.
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

// StopEntry stops a running time entry by setting its end time and updating its status to [model.StatusStopped].
//
// If there are any active pauses associated with the entry, they are marked as resumed with the current time.
//
// id is the unique identifier of the time entry to stop.
//
// Returns an error if the database operation to stop the entry or update pauses fails.
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

// DeleteEntry removes an entry with the specified ID from the database.
//
// It performs the following actions within a transaction:
//
//   - Deletes all associated pauses from the "pauses" table for the given entry ID.
//   - Deletes all associated tags from the "entry_tags" table for the given entry ID.
//   - Deletes the entry itself from the "entries" table.
//
// The transaction is committed upon successful deletion of all related data. If any step fails, the transaction is
// rolled back, and an error is returned.
//
// id: The identifier of the entry to delete.
//
// Returns an error if:
//   - A database transaction cannot be created.
//   - Any step in deleting related data or the entry itself fails.
//   - Committing the transaction fails.
func DeleteEntry(id string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete related pauses
	_, err = tx.Exec("DELETE FROM pauses WHERE entry_id = ?", id)
	if err != nil {
		return err
	}

	// Delete related tags
	_, err = tx.Exec("DELETE FROM entry_tags WHERE entry_id = ?", id)
	if err != nil {
		return err
	}

	// Delete entry
	_, err = tx.Exec("DELETE FROM entries WHERE id = ?", id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// PauseEntry creates a pause entry for the given `id` and updates its status to paused.
//
// A new ULID is generated for the pause entry, and the current timestamp is used as the pause time.
// Both the pause entry and the status update are executed within a database transaction to maintain atomicity.
//
// id:
//   - The unique identifier of the entry to pause.
//
// reason:
//   - The explanation or context for the pause. It is stored in the database for potential auditing or user reference.
//
// Returns an error if database operations fail, including transaction initiation, insertion, or update queries.
// The transaction is rolled back in any error case to ensure consistency.
func PauseEntry(id string, reason string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pauseID := model.NewULID()
	now := time.Now()

	_, err = tx.Exec("INSERT INTO pauses (id, entry_id, pause_time, reason) VALUES (?, ?, ?, ?)", pauseID, id, now, reason)
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE entries SET status = ? WHERE id = ?", model.StatusPaused, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ResumeEntry updates a paused timer entry to resume its activity by setting a resume time and changing its status.
//
// This function starts a database transaction and performs two updates:
//
// - It sets the `resume_time` in the `pauses` table for the specified entry where it is currently `NULL`.
// - It updates the `status` in the `entries` table to [model.StatusRunning] for the provided entry ID.
//
// If any update fails, the transaction is rolled back, and the corresponding error is returned.
//
// id:
//   - The identifier of the entry to resume, referencing its primary key in the `entries` table.
//
// Returns an error if there is any issue establishing the database transaction, executing the updates, or committing the changes.
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

// UpdateEntry updates an entry in the database with the given parameters.
//
// The function modifies the entry identified by id by updating its projectID, title, and optional startTime and endTime.
// If tagIDs are provided, the associated tags are updated by first clearing existing tags for the entry and then inserting
// the new tags. The function uses transactions to ensure atomicity and rolls back in case of any errors.
//
// If both startTime and endTime are provided, they are set in the entry. If only startTime is provided, the endTime remains
// unchanged. If neither are provided, the entry's timing details are not modified.
//
//   - id: The unique identifier of the entry to update.
//   - projectID: The identifier of the project associated with the entry.
//   - title: The new title of the entry.
//   - startTime, endTime: Optional timestamps for the entry's start and end times. Provide as pointers, or nil to skip updates.
//   - tagIDs: A slice of strings representing the tags to associate with the entry.
//
// Returns an error if the database operation fails, including transaction commit errors.
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

// ListEntriesOptions represents the parameters available for filtering and retrieving time entries.
//
// It includes options to limit the number of results, filter entries by project, associate specified tags,
// and constrain the time range by specifying start and end dates. Providing a ProjectID or a list of TagIDs
// narrows the results to entries associated with certain projects or tags. The From and To fields allow for
// date-based filtering of entries.
//
// The Limit field controls the maximum number of entries to be retrieved. If set to 0, all matching entries
// are retrieved.
//
// - Limit defines the maximum count of entries to return.
// - ProjectID specifies an optional project scope to filter entries.
// - TagIDs is a list of tag identifiers used to refine the search.
// - From restricts the entries to those starting on or after the specified time.
// - To restricts the entries to those starting before the specified time.
type ListEntriesOptions struct {
	Limit     int
	ProjectID *string
	TagIDs    []string
	From      *time.Time
	To        *time.Time
}

// ListEntries retrieves a list of time tracking entries based on the provided [ListEntriesOptions] filters.
//
// The function constructs a SQL query dynamically based on the filters in opts. These filters include project ID,
// tag IDs, time range (From and To), and the maximum number of records to retrieve (Limit). It ensures only the
// desired dataset is returned.
//
// The function populates additional details for each entry, such as the associated project, tags, and pauses.
// Errors encountered in database interactions or data mapping are returned.
//
//   - opts: Filters for retrieving entries, including time range, project, tags, and result limit.
//
// Returns:
//   - A slice of [model.Entry] containing the relevant entries, or an error if something goes wrong.
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

// repeatString concatenates the string s, n times, and returns the resulting string.
//
// If n is zero or negative, an empty string is returned.
//
// Parameters:
//   - s: The string to be repeated.
//   - n: The number of times the string is repeated.
//
// Returns:
//
//	A single string composed of s repeated n times.
func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// Pause operations

// GetPausesForEntry retrieves all pauses associated with a specific entry identified by entryID.
// A pause includes attributes like pause time, optional resume time, and a reason.
// Results are ordered by pause time in ascending order.
// entryID is the unique identifier of the entry for which pauses are queried.
// Returns a slice of [model.Pause], and nil if no pauses exist.
// Returns an error if the database query or data scanning fails.
func GetPausesForEntry(entryID string) ([]model.Pause, error) {
	rows, err := DB.Query(`
		SELECT id, entry_id, pause_time, resume_time, COALESCE(reason, 'Manual')
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
		if err := rows.Scan(&p.ID, &p.EntryID, &p.PauseTime, &resumeTime, &p.Reason); err != nil {
			return nil, err
		}
		if resumeTime.Valid {
			p.ResumeTime = &resumeTime.Time
		}
		pauses = append(pauses, p)
	}
	return pauses, rows.Err()
}

// DeletePause deletes a pause record from the database identified by the given id.
//
// The id parameter specifies the unique identifier of the pause to be deleted.
// Returns an error if the deletion fails, or nil if the operation is successful.
func DeletePause(id string) error {
	_, err := DB.Exec("DELETE FROM pauses WHERE id = ?", id)
	return err
}

// UpdatePause updates the pause record in the database identified by id with the specified pauseTime and resumeTime.
//
// The function executes an SQL update on the "pauses" table, setting the pause_time and resume_time fields. If resumeTime is nil, the field is updated to NULL.
//
//   - id: The unique identifier of the pause record to update.
//   - pauseTime: The timestamp when the pause started.
//   - resumeTime: An optional timestamp indicating when the pause ended. If nil, it is set to NULL.
//
// Returns an error if the database query fails, including cases such as connection issues or invalid SQL execution.
func UpdatePause(id string, pauseTime time.Time, resumeTime *time.Time) error {
	_, err := DB.Exec("UPDATE pauses SET pause_time = ?, resume_time = ? WHERE id = ?",
		pauseTime, resumeTime, id)
	return err
}

func CreatePause(entryID string, pauseTime time.Time, resumeTime *time.Time, reason string) (string, error) {
	pauseID := model.NewULID()
	_, err := DB.Exec(
		"INSERT INTO pauses (id, entry_id, pause_time, resume_time, reason) VALUES (?, ?, ?, ?, ?)",
		pauseID, entryID, pauseTime, resumeTime, reason)
	return pauseID, err
}

// Config operations

// GetConfig retrieves the configuration value associated with the given key from the database.
//
// If the key exists in the database, its value is returned. If the key does not exist, an empty string and no error
// are returned. An error is returned if the database query fails.
//
// - key: The configuration key to look up.
//
// Returns the value as a string if found, or an empty string if the key is not found. Returns an error if query fails.
func GetConfig(key string) (string, error) {
	var value string
	err := DB.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetConfig saves a persistent key-value pair in the configuration storage.
//
// This function ensures that the key and value provided are stored in the application's configuration database.
// If the key already exists, its value will be replaced.
//
// Errors can occur in the following scenarios:
//   - If there is an issue executing the database query.
//   - If the database connection [DB] is unavailable.
//
// Returns an error if the operation fails.
func SetConfig(key, value string) error {
	_, err := DB.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", key, value)
	return err
}

// ListConfig retrieves configuration data as a map from the database.
//
// The function executes a query to fetch all key-value pairs stored in the `config` table.
// Results are processed and returned as a map where each key represents a configuration setting.
//
// In case of a query failure or an issue while scanning the rows, an error is returned.
// The returned map is guaranteed to be nil if an error occurs.
//
// Returns:
//   - A map[string]string containing key-value pairs of configuration settings.
//   - An error if any issue occurs during query execution or row processing.
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

// GetProjectByName retrieves a project from the database by its name.
//
// It queries the "projects" table to find a project matching the specified name. If no project is found, it returns nil
// without an error. If a database error occurs, it returns the error.
//
// The returned project includes fields such as ID, Name, and CreatedAt.
//
// Parameters:
//   - name: The name of the project to look up.
//
// Returns:
//   - A pointer to the [model.Project] if found, or nil if no matching project exists.
//   - An error if the database query fails, other than no rows found.
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

// GetTagByName retrieves a tag from the database based on its name.
//
// The function searches for a tag with the given name in the `tags` table. If a matching tag is found,
// it returns a pointer to a [model.Tag] containing its details.
//
// If no matching tag is found, the function returns `nil` without an error. If a database query fails,
// an error is returned.
//
// Parameters:
//   - name: The name of the tag to retrieve.
//
// Returns:
//   - A pointer to the [model.Tag] struct containing the tag's details if found.
//   - `nil` if no tag matches the given name.
//   - An error if a database query fails.
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
