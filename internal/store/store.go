package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database for session, key-value, and activity tracking.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite database at dbPath and ensures all required
// tables exist. The caller should call Close when finished.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}
	return &Store{db: db}, nil
}

func createTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			user_id    INTEGER PRIMARY KEY,
			session_id TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS kv (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS analyzed_activities (
			activity_id TEXT PRIMARY KEY,
			analyzed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	return nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- Sessions ---

// GetSession returns the session ID for the given user, or "" if none exists.
func (s *Store) GetSession(userID int64) (string, error) {
	var sid string
	err := s.db.QueryRow(
		"SELECT session_id FROM sessions WHERE user_id = ?", userID,
	).Scan(&sid)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return sid, err
}

// SaveSession upserts the session ID for a user.
func (s *Store) SaveSession(userID int64, sessionID string) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (user_id, session_id, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET
		   session_id = excluded.session_id,
		   updated_at = CURRENT_TIMESTAMP`,
		userID, sessionID,
	)
	return err
}

// DeleteSession removes the session for the given user.
func (s *Store) DeleteSession(userID int64) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// --- Key-Value ---

// GetValue returns the value for a key, or "" if the key does not exist.
func (s *Store) GetValue(key string) (string, error) {
	var val string
	err := s.db.QueryRow(
		"SELECT value FROM kv WHERE key = ?", key,
	).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// SetValue upserts a key-value pair.
func (s *Store) SetValue(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO kv (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// --- Analyzed Activities ---

// GetAnalyzedActivities returns the set of all analyzed activity IDs.
func (s *Store) GetAnalyzedActivities() (map[string]struct{}, error) {
	rows, err := s.db.Query("SELECT activity_id FROM analyzed_activities")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = struct{}{}
	}
	return ids, rows.Err()
}

// MarkActivityAnalyzed records an activity as analyzed. Duplicates are ignored.
func (s *Store) MarkActivityAnalyzed(activityID string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO analyzed_activities (activity_id, analyzed_at)
		 VALUES (?, CURRENT_TIMESTAMP)`,
		activityID,
	)
	return err
}

// --- Convenience ---

// GetLastActivityCheck returns the date string stored under
// "last_activity_check", or today's UTC date if no value is stored.
func (s *Store) GetLastActivityCheck() (string, error) {
	val, err := s.GetValue("last_activity_check")
	if err != nil {
		return "", err
	}
	if val == "" {
		return time.Now().UTC().Format("2006-01-02"), nil
	}
	return val, nil
}
