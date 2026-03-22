package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS memories (
	id          TEXT PRIMARY KEY,
	content     TEXT NOT NULL,
	for_label   TEXT NOT NULL DEFAULT '',
	working_dir TEXT NOT NULL DEFAULT '',
	hostname    TEXT NOT NULL DEFAULT '',
	created_at  DATETIME NOT NULL,
	remind_at   DATETIME,
	reminded_at DATETIME,
	embedding   BLOB
);

CREATE INDEX IF NOT EXISTS idx_memories_remind_at  ON memories(remind_at) WHERE remind_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at DESC);

CREATE TABLE IF NOT EXISTS config (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// DB wraps a single SQLite connection and exposes Store and Config adapters
// that share the same underlying connection.
type DB struct {
	conn   *sql.DB
	Store  *Store
	Config *Config
}

// Open opens (or creates) the SQLite database at path, runs migrations,
// and returns a ready-to-use DB.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// WAL mode: better read/write concurrency for the daemon + CLI coexistence.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return &DB{
		conn:   conn,
		Store:  &Store{db: conn},
		Config: &Config{db: conn},
	}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}
