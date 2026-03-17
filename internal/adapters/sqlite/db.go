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
	type        TEXT NOT NULL DEFAULT 'note',
	tags        TEXT NOT NULL DEFAULT '[]',
	working_dir TEXT NOT NULL DEFAULT '',
	hostname    TEXT NOT NULL DEFAULT '',
	created_at  DATETIME NOT NULL,
	remind_at   DATETIME,
	reminded_at DATETIME,
	embedding   BLOB
);

CREATE INDEX IF NOT EXISTS idx_memories_type       ON memories(type);
CREATE INDEX IF NOT EXISTS idx_memories_remind_at  ON memories(remind_at) WHERE remind_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at DESC);

-- FTS5 full-text index for BM25 keyword search.
-- Content table mirrors the memories table so data is not duplicated.
-- Triggers below keep the FTS index in sync with the main table.
CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
	content,
	for_label,
	content='memories',
	content_rowid='rowid'
);

-- Sync triggers: keep memories_fts up-to-date with memories.
CREATE TRIGGER IF NOT EXISTS memories_fts_ai AFTER INSERT ON memories BEGIN
	INSERT INTO memories_fts(rowid, content, for_label)
	VALUES (new.rowid, new.content, new.for_label);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_ad AFTER DELETE ON memories BEGIN
	INSERT INTO memories_fts(memories_fts, rowid, content, for_label)
	VALUES ('delete', old.rowid, old.content, old.for_label);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_au AFTER UPDATE ON memories BEGIN
	INSERT INTO memories_fts(memories_fts, rowid, content, for_label)
	VALUES ('delete', old.rowid, old.content, old.for_label);
	INSERT INTO memories_fts(rowid, content, for_label)
	VALUES (new.rowid, new.content, new.for_label);
END;

-- Knowledge graph: named entities extracted from memory content.
CREATE TABLE IF NOT EXISTS entities (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	type       TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	UNIQUE(name, type)
);

CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);

-- Junction table linking memories to entities.
CREATE TABLE IF NOT EXISTS memory_entities (
	memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
	entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
	PRIMARY KEY (memory_id, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_memory_entities_entity ON memory_entities(entity_id);

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

	// Enable foreign key enforcement (required for ON DELETE CASCADE on memory_entities).
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	// Rebuild FTS index for any memories that predate the FTS table.
	// 'rebuild' is a no-op when the index is already current.
	if _, err := conn.Exec("INSERT INTO memories_fts(memories_fts) VALUES ('rebuild')"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("rebuild fts index: %w", err)
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
