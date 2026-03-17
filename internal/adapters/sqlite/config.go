package sqlite

import (
	"database/sql"

	"github.com/kunalsin9h/yaad/internal/ports"
)

// Compile-time interface check.
var _ ports.ConfigPort = (*Config)(nil)

// Config implements ports.ConfigPort backed by the config table in SQLite.
type Config struct {
	db *sql.DB
}

func (c *Config) Get(key string) (string, error) {
	var value string
	err := c.db.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (c *Config) Set(key, value string) error {
	_, err := c.db.Exec(
		"INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

func (c *Config) All() (map[string]string, error) {
	rows, err := c.db.Query("SELECT key, value FROM config ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}
