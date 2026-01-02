package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
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

func GetDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".tally"), nil
}

func Init() error {
	dataDir, err := GetDataDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "tally.db")
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	_, err = DB.Exec(schema)
	if err != nil {
		return err
	}

	// Initialize activity table with a single row
	_, err = DB.Exec(`INSERT OR IGNORE INTO activity (id, last_activity) VALUES (1, datetime('now'))`)
	return err
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
