package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tags (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS entries (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    title TEXT,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    status TEXT DEFAULT 'running',
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

CREATE TABLE IF NOT EXISTS entry_tags (
    entry_id TEXT,
    tag_id TEXT,
    PRIMARY KEY (entry_id, tag_id),
    FOREIGN KEY (entry_id) REFERENCES entries(id),
    FOREIGN KEY (tag_id) REFERENCES tags(id)
);

CREATE TABLE IF NOT EXISTS pauses (
    id TEXT PRIMARY KEY,
    entry_id TEXT NOT NULL,
    pause_time DATETIME NOT NULL,
    resume_time DATETIME,
    reason TEXT DEFAULT 'Manual',
    FOREIGN KEY (entry_id) REFERENCES entries(id)
);

CREATE TABLE IF NOT EXISTS config (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS activity (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    last_activity DATETIME
);
`

// GetDataDir returns the path to the application's data directory within the user's home directory.
//
// It determines the user's home directory using [os.UserHomeDir] and appends a folder named ".tally" to it.
// If the home directory cannot be determined, it returns an empty string and an error.
//
// Returns:
//   - A string representing the data directory path.
//   - An error if the user's home directory cannot be determined.
func GetDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".tally"), nil
}

// Init initializes the database and ensures the required schema is present.
//
// It creates necessary directories using [GetDataDir] and sets up the SQLite database at `tally.db`.
// Schema definitions and migrations are applied to establish or update the database structure.
//
// - If the `activity` table is empty, it inserts a default row.
// - Migrations are executed but may silently ignore errors related to redundant changes.
//
// Returns an error if directory creation or database initialization fails. Silent errors may occur for migrations.
func Init() error {
	dataDir, err := GetDataDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "tally.db")
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	_, err = DB.Exec(schema)
	if err != nil {
		return err
	}

	// Migrations
	migrations := []string{
		// Add reason column to pauses table
		`ALTER TABLE pauses ADD COLUMN reason TEXT DEFAULT 'Manual'`,
	}

	for _, m := range migrations {
		DB.Exec(m) // Ignore errors (column may already exist)
	}

	// Initialize activity table with a single row
	_, err = DB.Exec(`INSERT OR IGNORE INTO activity (id, last_activity) VALUES (1, datetime('now'))`)
	return err
}

// Close safely terminates the database connection held by DB.
//
// If DB is already nil, it does nothing and returns nil. Otherwise, it calls DB.Close() and returns any error that occurs.
func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
