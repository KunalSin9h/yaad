package domain

import "time"

// Memory is the core domain entity — one piece of remembered information.
type Memory struct {
	ID         string
	Content    string
	ForLabel   string // human context: "why did I save this?"
	WorkingDir string // cwd captured automatically at save time
	Hostname   string // machine identity
	CreatedAt  time.Time
	RemindAt   *time.Time // nil if not a reminder
	RemindedAt *time.Time // nil until notification fires
	Embedding  []float32  // vector for semantic search
}

// ListFilter controls which memories StoragePort.List returns.
type ListFilter struct {
	Limit         int
	OnlyReminders bool
}
