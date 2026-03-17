package ports

import (
	"context"
	"time"

	"github.com/kunalsin9h/yaad/internal/domain"
)

// StoragePort defines all persistence operations.
// Implemented by: adapters/sqlite.Store
type StoragePort interface {
	Save(ctx context.Context, m *domain.Memory) error
	GetByID(ctx context.Context, id string) (*domain.Memory, error)
	List(ctx context.Context, filter domain.ListFilter) ([]*domain.Memory, error)
	Delete(ctx context.Context, id string) error
	DeleteAll(ctx context.Context) (int64, error)

	// FindSimilar is the original pure-vector cosine similarity search.
	// Kept for backward compatibility and as a fallback.
	FindSimilar(ctx context.Context, embedding []float32, topK int) ([]*domain.Memory, error)

	// FindHybrid merges BM25 full-text search (SQLite FTS5) and cosine
	// similarity via Reciprocal Rank Fusion (RRF).  It outperforms
	// vector-only search on keyword-heavy queries (exact names, commands,
	// tags) and equals it on purely semantic queries.
	// query is the raw text used for BM25; embedding is its vector.
	FindHybrid(ctx context.Context, query string, embedding []float32, topK int) ([]*domain.Memory, error)

	// Entity / knowledge-graph methods.
	SaveEntities(ctx context.Context, memoryID string, entities []domain.Entity) error
	FindByEntities(ctx context.Context, names []string, topK int) ([]*domain.Memory, error)

	PendingReminders(ctx context.Context, before time.Time) ([]*domain.Memory, error)
	MarkReminded(ctx context.Context, id string) error
}
